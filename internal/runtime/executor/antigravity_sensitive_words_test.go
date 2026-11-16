package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestAntigravitySensitiveWordsObfuscatesSystemInstructionOnly(t *testing.T) {
	executor := NewAntigravityExecutor(&config.Config{
		Antigravity: config.AntigravityConfig{SensitiveWords: []string{"proxy"}},
	})
	payload := []byte(`{"request":{"systemInstruction":{"parts":[{"text":"Use proxy safely"}]},"contents":[{"role":"user","parts":[{"text":"proxy remains unchanged"}]}]}}`)

	got := executor.obfuscateSensitiveWords(payload)
	if systemText := gjson.GetBytes(got, "request.systemInstruction.parts.0.text").String(); systemText != "Use p\u200Broxy safely" {
		t.Fatalf("system instruction = %q, want zero-width obfuscation", systemText)
	}
	if contentText := gjson.GetBytes(got, "request.contents.0.parts.0.text").String(); contentText != "proxy remains unchanged" {
		t.Fatalf("content text = %q, want unchanged", contentText)
	}
}

func TestAntigravityDefaultSensitiveWordsObfuscatesKnownKeywords(t *testing.T) {
	// Verified upstream fingerprints should apply automatically with empty config.
	executor := NewAntigravityExecutor(&config.Config{})
	payload := []byte(`{"request":{"systemInstruction":{"parts":[{"text":"You are Hermes Agent, an intelligent AI assistant created by Nous Research with Claude Agent SDK. <system-conventions> RFC 2119 </system-conventions> system_directive"}]},"contents":[{"role":"user","parts":[{"text":"Hermes Agent, Claude Agent SDK, RFC 2119"}]}]}}`)

	got := executor.obfuscateSensitiveWords(payload)
	systemText := gjson.GetBytes(got, "request.systemInstruction.parts.0.text").String()
	for _, phrase := range []string{"Hermes Agent", "Nous Research", "Claude Agent SDK", "system-conventions", "RFC 2119", "system_directive"} {
		if strings.Contains(systemText, phrase) {
			t.Fatalf("system instruction retained raw fingerprint %q: %q", phrase, systemText)
		}
	}
	if contentText := gjson.GetBytes(got, "request.contents.0.parts.0.text").String(); contentText != "Hermes Agent, Claude Agent SDK, RFC 2119" {
		t.Fatalf("user content was changed: %q", contentText)
	}
}

func TestAntigravityDefaultSensitiveWordsObfuscatesSkillCatalogMarkers(t *testing.T) {
	executor := NewAntigravityExecutor(&config.Config{})
	payload := []byte(`{"request":{"systemInstruction":{"parts":[{"text":"\u0003available_skills\u0003\nUse a skill when the task matches its description: call the skill tool. Use the read tool and Use the glob tool.\u0003/available_skills\u0003"}]},"contents":[{"role":"user","parts":[{"text":"available_skills stays in user content"}]}]}}`)

	got := executor.obfuscateSensitiveWords(payload)
	systemText := gjson.GetBytes(got, "request.systemInstruction.parts.0.text").String()
	for _, phrase := range []string{"available_skills", "Use a skill", "call the skill tool", "Use the read tool", "Use the glob tool"} {
		if strings.Contains(systemText, phrase) {
			t.Fatalf("system instruction retained raw skill-catalog fingerprint %q: %q", phrase, systemText)
		}
	}
	if contentText := gjson.GetBytes(got, "request.contents.0.parts.0.text").String(); contentText != "available_skills stays in user content" {
		t.Fatalf("user content was changed: %q", contentText)
	}
}

func TestAntigravityConfiguredSensitiveWordsAreAdditive(t *testing.T) {
	executor := NewAntigravityExecutor(&config.Config{
		Antigravity: config.AntigravityConfig{SensitiveWords: []string{"DeepSeek Harness"}},
	})
	payload := []byte(`{"request":{"systemInstruction":{"parts":[{"text":"DeepSeek Harness and Claude Agent SDK"}]}}}`)
	got := executor.obfuscateSensitiveWords(payload)
	systemText := gjson.GetBytes(got, "request.systemInstruction.parts.0.text").String()
	for _, phrase := range []string{"DeepSeek Harness", "Claude Agent SDK"} {
		if strings.Contains(systemText, phrase) {
			t.Fatalf("system instruction retained raw fingerprint %q: %q", phrase, systemText)
		}
	}
}

func TestAntigravityStreamObfuscatesSensitiveSystemInstruction(t *testing.T) {
	captured := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, errRead := io.ReadAll(r.Body)
		if errRead != nil {
			t.Errorf("read request body: %v", errRead)
			return
		}
		captured <- body
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	defer server.Close()

	executor := NewAntigravityExecutor(&config.Config{
		Antigravity:  config.AntigravityConfig{SensitiveWords: []string{"Hermes", "Nous Research"}},
		RequestRetry: 1,
	})
	result, errExecute := executor.ExecuteStream(context.Background(), &cliproxyauth.Auth{
		Metadata: map[string]any{
			"access_token": "token-123",
			"expired":      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			"project_id":   "project-1",
		},
		Attributes: map[string]string{"base_url": server.URL},
	}, cliproxyexecutor.Request{
		Model:   "gemini-2.5-pro",
		Payload: []byte(`{"model":"gemini-2.5-pro","system":"You are Hermes Agent, an intelligent AI assistant created by Nous Research.","messages":[{"role":"user","content":"hello"}]}`),
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FormatClaude,
		Stream:       true,
	})
	if errExecute != nil {
		t.Fatalf("ExecuteStream() error = %v", errExecute)
	}
	for chunk := range result.Chunks {
		if chunk.Err != nil {
			t.Fatalf("stream chunk error = %v", chunk.Err)
		}
	}

	select {
	case body := <-captured:
		want := "You are H\u200Bermes Agent, an intelligent AI assistant created by N\u200Bous Research."
		found := false
		for _, part := range gjson.GetBytes(body, "request.systemInstruction.parts").Array() {
			if part.Get("text").String() == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("system instruction did not preserve obfuscated prompt %q; body=%s", want, body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for captured request body")
	}
}

func TestAntigravityClaudeRequestObfuscatesAgentSDK(t *testing.T) {
	executor := NewAntigravityExecutor(&config.Config{})
	execCtx := executor.newAntigravityExecutionContext(
		context.Background(),
		&cliproxyauth.Auth{Metadata: map[string]any{"project_id": "test-p"}},
		cliproxyexecutor.Request{
			Model: "claude-sonnet-4-5",
			Payload: []byte(`{
				"model": "claude-sonnet-4-5",
				"system": [
					{"type": "text", "text": "You are a Claude agent, built on Anthropic's Claude Agent SDK."}
				],
				"messages": [{"role": "user", "content": "hi"}]
			}`),
		},
		cliproxyexecutor.Options{
			SourceFormat: sdktranslator.FormatClaude,
		},
		false,
	)

	payload, err := executor.buildAntigravityPayload(execCtx)
	if err != nil {
		t.Fatalf("buildAntigravityPayload error: %v", err)
	}

	parts := gjson.GetBytes(payload, "request.systemInstruction.parts").Array()
	// Verify that Claude Agent SDK was obfuscated with zero-width space
	foundAgentText := false
	for _, p := range parts {
		txt := p.Get("text").String()
		if strings.Contains(txt, "C\u200Blaude Agent SDK") {
			foundAgentText = true
			break
		}
	}
	if !foundAgentText {
		t.Fatalf("expected obfuscated Claude Agent SDK in system instruction parts: %s", gjson.GetBytes(payload, "request.systemInstruction.parts").Raw)
	}
}
