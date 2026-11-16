package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	antigravityauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/antigravity"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/tidwall/gjson"
)

func TestAntigravityBuildRequest_SanitizesGeminiToolSchema(t *testing.T) {
	body := buildRequestBodyFromPayload(t, "gemini-2.5-pro")

	decl := extractFirstFunctionDeclaration(t, body)
	if _, ok := decl["parametersJsonSchema"]; ok {
		t.Fatalf("parametersJsonSchema should be renamed to parameters")
	}

	params, ok := decl["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("parameters missing or invalid type")
	}
	assertSchemaSanitizedAndPropertyPreserved(t, params)
}

func TestAntigravityBuildRequestDoesNotAddSyntheticPromptForGeminiFamily(t *testing.T) {
	cfg := &config.Config{}
	cfg.IdentityFingerprint.Antigravity = config.NormalizeAntigravityIdentityFingerprint(config.AntigravityIdentityFingerprintConfig{})
	executor := NewAntigravityExecutor(cfg)
	auth := &cliproxyauth.Auth{Metadata: map[string]any{"project_id": "test-project"}}

	req, _, err := executor.buildRequest(context.Background(), auth, "token", "gemini-4-hydrogen-medium", []byte(`{
		"request": {
			"contents": [{"role":"user","parts":[{"text":"hello"}]}],
			"systemInstruction": {"parts":[{"text":"You are a foreign coding agent."}]}
		}
	}`), true, "", "https://example.com", 0)
	if err != nil {
		t.Fatalf("buildRequest error: %v", err)
	}

	if got := req.Header.Get("User-Agent"); got != antigravityauth.ClientUserAgent {
		t.Fatalf("User-Agent = %q, want %q", got, antigravityauth.ClientUserAgent)
	}
	if got := req.Header.Get("X-Goog-Api-Client"); got != antigravityauth.APIClient {
		t.Fatalf("X-Goog-Api-Client = %q, want %q", got, antigravityauth.APIClient)
	}
	if got := req.Header.Get("Client-Metadata"); got != antigravityauth.ClientMetadata {
		t.Fatalf("Client-Metadata = %q, want %q", got, antigravityauth.ClientMetadata)
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body error: %v", err)
	}
	parts := gjson.GetBytes(raw, "request.systemInstruction.parts").Array()
	if len(parts) != 1 {
		t.Fatalf("system instruction parts = %d, want the caller's single prompt without synthetic Hub text: %s", len(parts), raw)
	}
	if got := parts[0].Get("text").String(); got != "You are a foreign coding agent." {
		t.Fatalf("original system instruction = %q, want it preserved", got)
	}
}

func TestAntigravityBuildRequestObfuscatesFinalWrappedSystemInstruction(t *testing.T) {
	executor := NewAntigravityExecutor(&config.Config{})
	auth := &cliproxyauth.Auth{Metadata: map[string]any{"project_id": "test-project"}}
	payload := []byte(`{
		"systemInstruction": {"parts": [{"text": "<available_skills> Use a skill when the task matches its description. </available_skills>"}]},
		"contents": [{"role": "user", "parts": [{"text": "hello"}]}]
	}`)
	req, body, err := executor.buildRequest(context.Background(), auth, "token", "gemini-4-hydrogen-medium", payload, true, "", "https://example.com", 0)
	if err != nil {
		t.Fatalf("buildRequest error: %v", err)
	}
	if got := string(body); strings.Contains(got, "available_skills") || strings.Contains(got, "Use a skill") {
		t.Fatalf("final upstream body retained skill fingerprint: %s", got)
	}
	if got := gjson.GetBytes(body, "request.systemInstruction.parts.0.text").String(); got != "" {
		t.Fatalf("final wrapped system instruction should be empty after catalog removal: %q", got)
	}
	if got := gjson.GetBytes(body, "request.contents.0.parts.0.text").String(); got != "hello" {
		t.Fatalf("user content changed: %q", got)
	}
	if got, err := io.ReadAll(req.Body); err != nil {
		t.Fatalf("read request body: %v", err)
	} else if !bytes.Equal(got, body) {
		t.Fatalf("request body differs from returned body: %s vs %s", got, body)
	}
}

func TestAntigravityBuildRequestKeepsHubPreambleForAntigravitySchema(t *testing.T) {
	cfg := &config.Config{}
	cfg.IdentityFingerprint.Antigravity = config.NormalizeAntigravityIdentityFingerprint(config.AntigravityIdentityFingerprintConfig{})
	executor := NewAntigravityExecutor(cfg)
	auth := &cliproxyauth.Auth{Metadata: map[string]any{"project_id": "test-project"}}

	req, _, err := executor.buildRequest(context.Background(), auth, "token", "claude-opus-4-6", []byte(`{
		"request": {
			"contents": [{"role":"user","parts":[{"text":"hello"}]}],
			"systemInstruction": {"parts":[{"text":"You are a foreign coding agent."}]}
		}
	}`), true, "", "https://example.com", 0)
	if err != nil {
		t.Fatalf("buildRequest error: %v", err)
	}
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body error: %v", err)
	}
	parts := gjson.GetBytes(raw, "request.systemInstruction.parts").Array()
	if len(parts) < 3 || parts[0].Get("text").String() != systemInstruction || parts[2].Get("text").String() != "You are a foreign coding agent." {
		t.Fatalf("Antigravity schema system instruction = %s, want Hub preamble followed by caller prompt", raw)
	}
}

func TestAntigravityBuildRequest_SanitizesAntigravityToolSchema(t *testing.T) {
	body := buildRequestBodyFromPayload(t, "claude-opus-4-6")

	decl := extractFirstFunctionDeclaration(t, body)
	params, ok := decl["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("parameters missing or invalid type")
	}
	assertSchemaSanitizedAndPropertyPreserved(t, params)
}

func buildRequestBodyFromPayload(t *testing.T, modelName string) map[string]any {
	t.Helper()

	executor := &AntigravityExecutor{}
	auth := &cliproxyauth.Auth{Metadata: map[string]any{"project_id": "test-project"}}
	payload := []byte(`{
		"request": {
			"tools": [
				{
					"function_declarations": [
						{
							"name": "tool_1",
							"parametersJsonSchema": {
								"$schema": "http://json-schema.org/draft-07/schema#",
								"$id": "root-schema",
								"type": "object",
								"properties": {
									"$id": {"type": "string"},
									"arg": {
										"type": "object",
										"prefill": "hello",
										"properties": {
											"mode": {
												"type": "string",
												"enum": ["a", "b"],
												"enumTitles": ["A", "B"]
											}
										}
									}
								},
								"patternProperties": {
									"^x-": {"type": "string"}
								}
							}
						}
					]
				}
			]
		}
	}`)

	req, _, err := executor.buildRequest(context.Background(), auth, "token", modelName, payload, false, "", "https://example.com", 0)
	if err != nil {
		t.Fatalf("buildRequest error: %v", err)
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body error: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal request body error: %v, body=%s", err, string(raw))
	}
	return body
}

func extractFirstFunctionDeclaration(t *testing.T, body map[string]any) map[string]any {
	t.Helper()

	request, ok := body["request"].(map[string]any)
	if !ok {
		t.Fatalf("request missing or invalid type")
	}
	tools, ok := request["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools missing or empty")
	}
	tool, ok := tools[0].(map[string]any)
	if !ok {
		t.Fatalf("first tool invalid type")
	}
	decls, ok := tool["function_declarations"].([]any)
	if !ok || len(decls) == 0 {
		t.Fatalf("function_declarations missing or empty")
	}
	decl, ok := decls[0].(map[string]any)
	if !ok {
		t.Fatalf("first function declaration invalid type")
	}
	return decl
}

func assertSchemaSanitizedAndPropertyPreserved(t *testing.T, params map[string]any) {
	t.Helper()

	if _, ok := params["$id"]; ok {
		t.Fatalf("root $id should be removed from schema")
	}
	if _, ok := params["patternProperties"]; ok {
		t.Fatalf("patternProperties should be removed from schema")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing or invalid type")
	}
	if _, ok := props["$id"]; !ok {
		t.Fatalf("property named $id should be preserved")
	}

	arg, ok := props["arg"].(map[string]any)
	if !ok {
		t.Fatalf("arg property missing or invalid type")
	}
	if _, ok := arg["prefill"]; ok {
		t.Fatalf("prefill should be removed from nested schema")
	}

	argProps, ok := arg["properties"].(map[string]any)
	if !ok {
		t.Fatalf("arg.properties missing or invalid type")
	}
	mode, ok := argProps["mode"].(map[string]any)
	if !ok {
		t.Fatalf("mode property missing or invalid type")
	}
	if _, ok := mode["enumTitles"]; ok {
		t.Fatalf("enumTitles should be removed from nested schema")
	}
}
