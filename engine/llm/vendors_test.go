package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Exercise actual HTTP adapters over two turns: model tool call, then tool result.
// Both streaming and non-streaming must preserve each vendor's reasoning state.
func TestVendorToolRoundTrip(t *testing.T) {
	for _, format := range []Format{FormatGrok, FormatDeepSeek} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", format, stream), func(t *testing.T) {
				var calls atomic.Int32
				cipher := map[string]any{"type": "reasoning", "id": "reason-1", "encrypted_content": "opaque-test-ciphertext", "summary": []any{}}
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					call := calls.Add(1)
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if r.Header.Get("Authorization") != "Bearer vendor-test-key" || body["model"] != DefaultModel(format) || body["stream"] != stream {
						t.Errorf("incorrect provider request: path=%s model=%v stream=%v", r.URL.Path, body["model"], body["stream"])
					}
					wantPath := "/chat/completions"
					if format == FormatGrok {
						wantPath = "/responses"
						if body["store"] != false || body["thinking"] != nil {
							t.Error("Grok must use stateless Responses without a thinking field")
						}
					}
					if r.URL.Path != wantPath {
						t.Errorf("path=%s, want %s", r.URL.Path, wantPath)
					}
					if call == 2 {
						serialized, _ := json.Marshal(body)
						if !strings.Contains(string(serialized), "tool-evidence") {
							t.Error("tool output was not replayed")
						}
						if format == FormatGrok {
							items := body["input"].([]any)
							found := false
							for _, raw := range items {
								item := raw.(map[string]any)
								if item["type"] == "reasoning" {
									got, _ := json.Marshal(item)
									want, _ := json.Marshal(cipher)
									found = string(got) == string(want)
								}
							}
							if !found {
								t.Error("encrypted reasoning was lost or changed")
							}
						} else if !strings.Contains(string(serialized), `"reasoning_content":"retained-test-reasoning"`) {
							t.Error("DeepSeek reasoning_content was not replayed")
						}
					}
					if format == FormatGrok {
						if stream {
							w.Header().Set("Content-Type", "text/event-stream")
							frame := func(kind string, data map[string]any) {
								data["type"] = kind
								b, _ := json.Marshal(data)
								fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, b)
							}
							if call == 1 {
								frame("response.output_item.done", map[string]any{"item": cipher})
								frame("response.output_item.added", map[string]any{"item": map[string]any{"type": "function_call", "call_id": "call-1", "name": "probe"}})
								frame("response.function_call_arguments.delta", map[string]any{"delta": `{"value":"test"}`})
							} else {
								frame("response.output_text.delta", map[string]any{"delta": "Complete"})
							}
							frame("response.completed", map[string]any{"response": map[string]any{"status": "completed", "usage": map[string]int{"input_tokens": 10, "output_tokens": 5}}})
						} else {
							output := []any{map[string]any{"type": "message", "content": []any{map[string]string{"type": "output_text", "text": "Complete"}}}}
							if call == 1 {
								output = []any{cipher, map[string]any{"type": "function_call", "call_id": "call-1", "name": "probe", "arguments": `{"value":"test"}`}}
							}
							json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": output, "usage": map[string]int{"input_tokens": 10, "output_tokens": 5}})
						}
					} else {
						message := map[string]any{"role": "assistant", "content": "Complete"}
						finish := "stop"
						if call == 1 {
							message["content"] = ""
							message["reasoning_content"] = "retained-test-reasoning"
							message["tool_calls"] = []any{map[string]any{"index": 0, "id": "call-1", "type": "function", "function": map[string]string{"name": "probe", "arguments": `{"value":"test"}`}}}
							finish = "tool_calls"
						}
						if stream {
							w.Header().Set("Content-Type", "text/event-stream")
							b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": message}}})
							fmt.Fprintf(w, "data: %s\n\n", b)
							b, _ = json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5}})
							fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
						} else {
							json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": finish}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5}})
						}
					}
				}))
				defer upstream.Close()
				p, err := NewProvider(Config{Format: format, BaseURL: upstream.URL, APIKey: "vendor-test-key", MaxRetries: -1})
				if err != nil {
					t.Fatal(err)
				}
				req := CompletionRequest{Messages: []Message{UserText("Run the probe")}, Tools: []ToolSchema{{Name: "probe", InputSchema: map[string]any{"type": "object"}}}}
				run := func() Message {
					if !stream {
						m, _, _, err := p.Complete(context.Background(), req)
						if err != nil {
							t.Fatal(err)
						}
						return m
					}
					acc := NewAccumulator()
					for ev, err := range p.Stream(context.Background(), req) {
						if err != nil {
							t.Fatal(err)
						}
						acc.Add(ev)
					}
					return acc.Message()
				}
				first := run()
				if len(first.ToolUses()) != 1 {
					t.Fatalf("missing tool call: %+v", first)
				}
				// Exercise transcript persistence before continuing the conversation.
				saved, _ := json.Marshal(first)
				var restored Message
				if err := json.Unmarshal(saved, &restored); err != nil {
					t.Fatal(err)
				}
				req.Messages = append(req.Messages, restored, Message{Role: RoleUser, Content: []ContentBlock{ToolResultText("call-1", "tool-evidence", false)}})
				if final := run(); final.Text() != "Complete" {
					t.Fatalf("unexpected result: %+v", final)
				}
				if calls.Load() != 2 {
					t.Fatalf("calls=%d, want 2", calls.Load())
				}
			})
		}
	}
}

func TestOpaqueReasoningDoesNotCrossProviders(t *testing.T) {
	msgs := []Message{{Role: RoleAssistant, Content: []ContentBlock{{Type: BlockReasoning, ProviderFormat: FormatGrok, ProviderItem: json.RawMessage(`{"type":"reasoning","encrypted_content":"opaque"}`)}, TextBlock("Hello")}}}
	for _, enabled := range []bool{false, true} {
		filtered := filterThinkingBlocks(msgs, enabled)
		if len(filtered) != 1 || len(filtered[0].Content) != 1 || filtered[0].Text() != "Hello" {
			t.Fatal("opaque state leaked into Anthropic history")
		}
	}
	if len(toResponsesInput(msgs, FormatOpenAIResponses)) != 1 {
		t.Fatal("Grok state leaked into another Responses provider")
	}
}
