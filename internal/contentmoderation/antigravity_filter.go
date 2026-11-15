package contentmoderation

import (
	"context"
	"net/http"
	"strings"

	antigravityfilter "github.com/ititrica/cpa-plugin-antigravity-coding-filter/filter"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type antigravityFilterConfig struct {
	mode               antigravityfilter.Mode
	useDefaultKeywords bool
	customMappings     map[string]string
}

// SetAntigravityCodingFilter configures the v6-native compatibility path. It is
// intentionally separate from the dynamic plugin lifecycle: CliRelay v6 does
// not expose the v7 plugin ABI, but it can run the same filter after selecting
// an Antigravity credential.
func (m *RuntimeModerator) SetAntigravityCodingFilter(enabled bool, mode string, useDefaultKeywords bool, customMappings map[string]string) {
	if m == nil {
		return
	}
	m.antigravityFilterEnabled = enabled
	filterMode := antigravityfilter.ModeBlock
	if strings.EqualFold(strings.TrimSpace(mode), string(antigravityfilter.ModeRewrite)) {
		filterMode = antigravityfilter.ModeRewrite
	}
	m.antigravityFilter = antigravityFilterConfig{
		mode:               filterMode,
		useDefaultKeywords: useDefaultKeywords,
		customMappings:     cloneStringMap(customMappings),
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func (m *RuntimeModerator) antigravityFilterConfig() antigravityfilter.Config {
	cfg := m.antigravityFilter
	mappings := make([]antigravityfilter.Mapping, 0, len(cfg.customMappings))
	for match, replacement := range cfg.customMappings {
		mappings = append(mappings, antigravityfilter.Mapping{Match: match, Replacement: replacement})
	}
	return antigravityfilter.Config{
		Mode:               cfg.mode,
		UseDefaultKeywords: cfg.useDefaultKeywords,
		CustomMappings:     mappings,
	}
}

func (m *RuntimeModerator) moderateAntigravity(auth *coreauth.Auth, opts cliproxyexecutor.Options) coreauth.RequestModerationResult {
	if m == nil || !m.antigravityFilterEnabled || auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "antigravity") {
		return coreauth.RequestModerationResult{}
	}
	if m.antigravityFilter.mode != antigravityfilter.ModeBlock {
		return coreauth.RequestModerationResult{}
	}
	blocked, signal := antigravityfilter.Classify(opts.OriginalRequest, m.antigravityFilterConfig())
	if !blocked {
		return coreauth.RequestModerationResult{}
	}
	message := "request blocked because it matches a configured non-Antigravity coding software keyword"
	if signal != "" {
		message += " (" + signal + ")"
	}
	return coreauth.RequestModerationResult{Blocked: true, Message: message, HTTPStatus: http.StatusForbidden}
}

// RewriteRequest runs only after the manager selected an Antigravity account.
// This preserves the plugin's provider gate even when the request can fail over
// across different providers.
func (m *RuntimeModerator) RewriteRequest(_ context.Context, auth *coreauth.Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) coreauth.RequestGuardResult {
	if m == nil || !m.antigravityFilterEnabled || m.antigravityFilter.mode != antigravityfilter.ModeRewrite || auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "antigravity") {
		return coreauth.RequestGuardResult{}
	}
	body := opts.OriginalRequest
	if len(body) == 0 {
		body = req.Payload
	}
	rewritten, changed := antigravityfilter.Rewrite(body, m.antigravityFilterConfig())
	if !changed {
		return coreauth.RequestGuardResult{}
	}
	return coreauth.RequestGuardResult{OriginalRequest: rewritten, Payload: rewritten}
}
