package contentmoderation

import (
	"context"
	"net/http"
	"strings"
	"testing"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func TestAntigravityCodingFilterBlocksOnlySelectedProvider(t *testing.T) {
	moderator := NewRequestModerator(nil, nil)
	moderator.SetAntigravityCodingFilter(true, "block", true, nil)
	opts := cliproxyexecutor.Options{OriginalRequest: []byte(`{"system":"You are Codex"}`)}

	blocked := moderator.Moderate(context.Background(), &coreauth.Auth{Provider: "antigravity"}, opts)
	if !blocked.Blocked || blocked.HTTPStatus != http.StatusForbidden {
		t.Fatalf("Antigravity result = %+v, want a 403 block", blocked)
	}
	allowed := moderator.Moderate(context.Background(), &coreauth.Auth{Provider: "openai"}, opts)
	if allowed.Blocked {
		t.Fatalf("non-Antigravity provider was blocked: %+v", allowed)
	}
}

func TestAntigravityCodingFilterRewritePreservesUserMessage(t *testing.T) {
	moderator := NewRequestModerator(nil, nil)
	moderator.SetAntigravityCodingFilter(true, "rewrite", true, nil)
	body := []byte(`{"messages":[{"role":"system","content":"Use Codex"},{"role":"user","content":"I use Codex"}]}`)
	rewritten := moderator.RewriteRequest(context.Background(), &coreauth.Auth{Provider: "antigravity"}, cliproxyexecutor.Request{Payload: body}, cliproxyexecutor.Options{OriginalRequest: body})
	if rewritten.Blocked || len(rewritten.Payload) == 0 {
		t.Fatalf("rewrite result = %+v, want payload", rewritten)
	}
	text := string(rewritten.Payload)
	if !strings.Contains(text, `"content":"Use Antigravity"`) || !strings.Contains(text, `"content":"I use Codex"`) {
		t.Fatalf("rewrite changed the wrong fields: %s", text)
	}
}
