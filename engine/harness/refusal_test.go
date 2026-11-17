package harness_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"antifox/engine/harness"
	"antifox/engine/llm"
)

func TestProviderRefusalIsTerminal(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		for _, withText := range []bool{false, true} {
			t.Run(fmt.Sprintf("sync=%v/text=%v", synchronous, withText), func(t *testing.T) {
				var calls atomic.Int32
				const refusalText = "I cannot complete this request."
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if synchronous {
						content := []map[string]string{}
						if withText {
							content = append(content, map[string]string{"type": "text", "text": refusalText})
						}
						json.NewEncoder(w).Encode(map[string]any{
							"type": "message", "content": content, "stop_reason": "refusal",
							"usage": map[string]int{"input_tokens": 42, "output_tokens": 0},
						})
						return
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":42,\"output_tokens\":0}}}\n\n")
					if withText {
						fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", refusalText)
					}
					fmt.Fprint(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"refusal\"},\"usage\":{\"output_tokens\":0}}\n\ndata: {\"type\":\"message_stop\"}\n\n")
				}))
				defer upstream.Close()
				provider, err := llm.NewProvider(llm.Config{
					Format: llm.FormatAnthropic, BaseURL: upstream.URL, APIKey: "test-key", Model: "test-model",
				})
				if err != nil {
					t.Fatal(err)
				}
				var result *harness.Terminal
				for ev, err := range harness.Query(context.Background(), harness.QueryInput{
					Provider: provider, Messages: []llm.Message{llm.UserText("test")}, NonStreaming: synchronous,
				}, harness.QueryDeps{}) {
					if err != nil {
						t.Fatal(err)
					}
					if ev.Kind == harness.KindResult {
						result = ev.Terminal
					}
				}
				if result == nil || result.Reason != harness.ReasonModelRefusal || !errors.Is(result.Err, harness.ErrModelRefusal) {
					t.Fatalf("expected terminal refusal, got %+v", result)
				}
				if calls.Load() != 1 {
					t.Fatalf("refusal was retried: %d requests", calls.Load())
				}
				if result.Usage.InputTokens != 42 || result.Usage.OutputTokens != 0 {
					t.Fatalf("usage was lost: %+v", result.Usage)
				}
				if withText && result.Text != refusalText {
					t.Fatalf("refusal explanation was lost: %q", result.Text)
				}
			})
		}
	}
}
