package server

import (
	"os"
	"strings"

	"antifox/agent"
	"antifox/engine/llm"
)

const providerKeyRequired = "Enter an API key for the selected provider and API base URL; an existing key cannot be reused after either changes."

// A blank key may reuse a saved credential only for its original destination.
func sameLLMCredentialScope(a, b agent.Config) bool {
	base := func(c agent.Config) string {
		if c.BaseURL == "" {
			env := map[llm.Format]string{llm.FormatAnthropic: "ANTHROPIC_BASE_URL", llm.FormatOpenAI: "OPENAI_BASE_URL", llm.FormatOpenAIResponses: "OPENAI_BASE_URL", llm.FormatGrok: "XAI_BASE_URL", llm.FormatDeepSeek: "DEEPSEEK_BASE_URL"}
			if configured := strings.TrimSpace(os.Getenv(env[c.Format])); configured != "" {
				return agent.ConfigFrom(c.Provider(), c.Model, configured, "", "").BaseURL
			}
			return llm.DefaultBaseURL(c.Format)
		}
		return strings.TrimRight(c.BaseURL, "/")
	}
	return a.Format == b.Format && base(a) == base(b)
}
