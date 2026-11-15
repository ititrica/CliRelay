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
