package executor

import (
	"context"
	"net/http"
	"strings"

	antigravityauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/antigravity"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/identityfingerprint"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func antigravityIdentityFingerprint(cfg *config.Config, auth *cliproxyauth.Auth, ctx context.Context) (config.AntigravityIdentityFingerprintConfig, *identityfingerprint.LearnedRecord, bool) {
	if cfg == nil || !cfg.IdentityFingerprint.Antigravity.Enabled {
		return config.DefaultAntigravityIdentityFingerprint(), nil, false
	}
	learned := observeRuntimeIdentityFingerprint(identityfingerprint.ProviderAntigravity, auth, ctx)
	resolved, _ := identityfingerprint.ResolveAntigravity(cfg.IdentityFingerprint.Antigravity, learned)
	return resolved, learned, true
}

func applyAntigravityIdentityFingerprintHeaders(headers http.Header, fp config.AntigravityIdentityFingerprintConfig) {
	if headers == nil || !fp.Enabled {
		return
	}
	// These are the headers emitted by Antigravity Hub's CloudCode client. Keep
	// them together with the Hub User-Agent so a foreign Agent cannot arrive as
	// a hybrid identity with Gemini CLI metadata or no client metadata.
	headers.Set("X-Goog-Api-Client", antigravityauth.APIClient)
	headers.Set("Client-Metadata", antigravityauth.ClientMetadata)
	if ua := strings.TrimSpace(fp.UserAgent); ua != "" {
		headers.Set("User-Agent", ua)
	}
	for key, value := range fp.CustomHeaders {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			headers.Set(key, value)
		}
	}
}

// applyAntigravityIdentityFingerprint applies the same Hub identity to any
// request that enters the executor through the generic transport interface.
// Generation and count-token requests are built locally, but model discovery
// and callers of Manager.NewHttpRequest use PrepareRequest instead. Keeping the
// operation here prevents those less common paths from going upstream with a
// missing User-Agent, X-Goog-Api-Client, or Client-Metadata header.
func (e *AntigravityExecutor) applyAntigravityIdentityFingerprint(ctx context.Context, auth *cliproxyauth.Auth, headers http.Header) {
	if e == nil || e.cfg == nil || headers == nil {
		return
	}
	if fp, _, enabled := antigravityIdentityFingerprint(e.cfg, auth, ctx); enabled {
		applyAntigravityIdentityFingerprintHeaders(headers, fp)
	}
}
