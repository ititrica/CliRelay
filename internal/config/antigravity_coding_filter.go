package config

// AntigravityCodingFilterConfig controls the native v6-compatible integration
// of the Antigravity Coding Filter. The v7 dynamic plugin uses the same fields.
type AntigravityCodingFilterConfig struct {
	Enabled            bool              `yaml:"enabled" json:"enabled"`
	Mode               string            `yaml:"mode,omitempty" json:"mode,omitempty"`
	UseDefaultKeywords bool              `yaml:"use-default-keywords" json:"use-default-keywords"`
	CustomMappings     map[string]string `yaml:"custom-mappings,omitempty" json:"custom-mappings,omitempty"`
}
