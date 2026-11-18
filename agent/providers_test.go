package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"antifox/engine/llm"
)

func TestVendorConfigAndEnvironment(t *testing.T) {
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "XAI_API_KEY", "DEEPSEEK_API_KEY", "ANTIFOX_LLM_BASE_URL", "ANTIFOX_LLM_MODEL", "ANTIFOX_LLM_PROXY", "XAI_BASE_URL", "DEEPSEEK_BASE_URL"} {
		t.Setenv(key, "")
	}
	t.Setenv("ANTHROPIC_API_KEY", "unrelated-anthropic-key")
	t.Setenv("OPENAI_API_KEY", "unrelated-openai-key")
	for _, tc := range []struct{ name, key, model, base, endpoint string }{
		{"grok", "XAI_API_KEY", "grok-4.7", "https://api.x.ai/v1", "/responses"},
		{"deepseek", "DEEPSEEK_API_KEY", "deepseek-flash", "https://api.deepseek.com", "/chat/completions"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ANTIFOX_LLM_PROVIDER", tc.name)
			if _, ok := FromEnv(); ok {
				t.Fatal("reused a different vendor's environment credential")
			}
			t.Setenv(tc.key, "matching-vendor-key")
			cfg, ok := FromEnv()
			if !ok || cfg.Provider() != tc.name || cfg.Model != tc.model || cfg.BaseURL != tc.base || cfg.APIKey != "matching-vendor-key" {
				t.Fatal("incorrect environment defaults")
			}
			custom := ConfigFrom(tc.name, "custom-model", "https://gateway.example/v1"+tc.endpoint+"/", "key", "")
			if custom.BaseURL != "https://gateway.example/v1" || custom.Model != "custom-model" {
				t.Fatalf("custom endpoint/model lost: %+v", custom)
			}
		})
	}
	if cfg := ConfigFrom("xai", "", "", "", ""); cfg.Format != llm.FormatGrok {
		t.Fatal("xai alias not normalized")
	}
	if _, err := ConfigFrom("unknown-vendor", "", "", "", "").NewProvider(); err == nil {
		t.Fatal("unknown provider silently accepted")
	}
}

func TestGrokReasoningOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Reasoning struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
			Thinking any `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Reasoning.Effort != "xhigh" || body.Thinking != nil {
			t.Errorf("unsupported Grok reasoning parameters: %+v", body)
		}
		w.Write([]byte(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"OK"}]}]}`))
	}))
	defer server.Close()
	cfg := ConfigFrom("grok", "", server.URL, "test-key", "")
	cfg.ReasoningEffort = "max"
	p, err := cfg.NewProvider()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := p.Complete(context.Background(), llm.CompletionRequest{Messages: []llm.Message{llm.UserText("Hello")}}); err != nil {
		t.Fatal(err)
	}
	cfg.ReasoningEffort = "off"
	if _, err := cfg.NewProvider(); err == nil {
		t.Fatal("unsupported disabled Grok reasoning was accepted")
	}
}
