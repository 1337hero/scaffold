package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompletionJSON_NativeFormat(t *testing.T) {
	var gotBody openAIChatCompletionRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"key":"value"}`}},
			},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, false, true, false)
	result, err := client.CompletionJSON(context.Background(), "gpt-4", "system", "user prompt", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"key":"value"}` {
		t.Fatalf("got %q, want %q", result, `{"key":"value"}`)
	}
	if gotBody.ResponseFormat == nil || gotBody.ResponseFormat["type"] != "json_object" {
		t.Fatalf("expected response_format with json_object, got %v", gotBody.ResponseFormat)
	}
}

func TestCompletionJSON_NonNativeFormat(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"key":"value"}`}},
			},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, false, false, false)
	_, err := client.CompletionJSON(context.Background(), "gpt-4", "system", "user prompt", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(rawBody), "response_format") {
		t.Fatalf("request body should not contain response_format, got: %s", rawBody)
	}
}

func TestCompletionText_TrimmedOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "  hello world  "}},
			},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, false, false, false)
	result, err := client.CompletionText(context.Background(), "gpt-4", "system", "user prompt", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("got %q, want %q", result, "hello world")
	}
}

func TestRespond_ToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{
					"content": "I'll look that up.",
					"tool_calls": []map[string]any{
						{
							"id":   "call_123",
							"type": "function",
							"function": map[string]any{
								"name":      "search",
								"arguments": `{"query":"weather"}`,
							},
						},
					},
				}},
			},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, true, false, false)
	resp, err := client.Respond(context.Background(), ToolUseRequest{
		Model:     "gpt-4",
		MaxTokens: 100,
		Messages:  []RespondMessage{{Role: "user", Text: "what's the weather?"}},
		Tools: []ToolDefinition{{
			Name:        "search",
			Description: "search the web",
			InputSchema: map[string]any{"properties": map[string]any{"query": map[string]any{"type": "string"}}},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "I'll look that up." {
		t.Fatalf("got text %q, want %q", resp.Text, "I'll look that up.")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Fatalf("got ID %q, want %q", tc.ID, "call_123")
	}
	if tc.Name != "search" {
		t.Fatalf("got Name %q, want %q", tc.Name, "search")
	}
	var input map[string]string
	json.Unmarshal(tc.Input, &input)
	if input["query"] != "weather" {
		t.Fatalf("got input query %q, want %q", input["query"], "weather")
	}
}

func TestRespond_ToolUseNotSupported(t *testing.T) {
	client := newOpenAIClient("http://unused", "key", 0, false, false, false)
	_, err := client.Respond(context.Background(), ToolUseRequest{Model: "gpt-4"})
	if err == nil || !strings.Contains(err.Error(), "does not support tool use") {
		t.Fatalf("expected tool use error, got: %v", err)
	}
}

func TestRespond_ToolResultsInHistory(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "done"}},
			},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, true, false, false)
	_, err := client.Respond(context.Background(), ToolUseRequest{
		Model:     "gpt-4",
		MaxTokens: 100,
		Messages: []RespondMessage{
			{Role: "user", Text: "search for weather"},
			{
				Role: "user",
				ToolResults: []ToolResult{{
					ToolUseID: "call_abc",
					Content:   "sunny 72F",
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages, ok := gotBody["messages"].([]any)
	if !ok {
		t.Fatalf("messages not found in request body")
	}

	var foundToolMsg bool
	for _, msg := range messages {
		m := msg.(map[string]any)
		if m["role"] == "tool" {
			foundToolMsg = true
			if m["tool_call_id"] != "call_abc" {
				t.Fatalf("got tool_call_id %v, want %q", m["tool_call_id"], "call_abc")
			}
			if m["content"] != "sunny 72F" {
				t.Fatalf("got content %v, want %q", m["content"], "sunny 72F")
			}
		}
	}
	if !foundToolMsg {
		t.Fatalf("no tool message found in request body")
	}
}

func TestRespond_ToolResultErrorPrefix(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &gotBody)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, true, false, false)
	_, err := client.Respond(context.Background(), ToolUseRequest{
		Model:     "gpt-4",
		MaxTokens: 100,
		Messages: []RespondMessage{
			{
				Role:      "assistant",
				ToolCalls: []ToolCall{{ID: "call_1", Name: "fn", Input: json.RawMessage(`{}`)}},
				ToolResults: []ToolResult{{
					ToolUseID: "call_1",
					Content:   "not found",
					IsError:   true,
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages, ok := gotBody["messages"].([]any)
	if !ok {
		t.Fatalf("messages not present in request body")
	}
	for _, msg := range messages {
		m := msg.(map[string]any)
		if m["role"] == "tool" {
			content := m["content"].(string)
			if !strings.HasPrefix(content, "Error: ") {
				t.Fatalf("expected error prefix, got %q", content)
			}
		}
	}
}

func TestChatCompletion_ServerError500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": ""},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, false, false, false)
	_, err := client.CompletionText(context.Background(), "gpt-4", "", "hello", 100)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error should mention status 500, got: %v", err)
	}
}

func TestChatCompletion_QuotaExceeded429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "quota exceeded"},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, false, false, false)
	_, err := client.CompletionText(context.Background(), "gpt-4", "", "hello", 100)
	if err == nil {
		t.Fatal("expected error for 429 status")
	}
	if !strings.Contains(err.Error(), "quota exceeded") {
		t.Fatalf("error should contain 'quota exceeded', got: %v", err)
	}
}

func TestChatCompletion_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "test-key", 0, false, false, false)
	_, err := client.CompletionText(context.Background(), "gpt-4", "", "hello", 100)
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if !strings.Contains(err.Error(), "empty response choices") {
		t.Fatalf("expected 'empty response choices' error, got: %v", err)
	}
}

func TestChatCompletion_AuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	client := newOpenAIClient(srv.URL, "sk-secret", 0, false, false, false)
	client.CompletionText(context.Background(), "gpt-4", "", "hello", 100)
	if gotAuth != "Bearer sk-secret" {
		t.Fatalf("got Authorization %q, want %q", gotAuth, "Bearer sk-secret")
	}
}

func TestExtractOpenAIContent(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := extractOpenAIContent(nil); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		if got := extractOpenAIContent("  hello  "); got != "hello" {
			t.Fatalf("got %q, want %q", got, "hello")
		}
	})

	t.Run("array_of_content_blocks", func(t *testing.T) {
		blocks := []any{
			map[string]any{"text": "first"},
			map[string]any{"text": "second"},
		}
		got := extractOpenAIContent(blocks)
		if got != "first\nsecond" {
			t.Fatalf("got %q, want %q", got, "first\nsecond")
		}
	})

	t.Run("array_skips_non_text", func(t *testing.T) {
		blocks := []any{
			map[string]any{"type": "image"},
			map[string]any{"text": "only this"},
		}
		got := extractOpenAIContent(blocks)
		if got != "only this" {
			t.Fatalf("got %q, want %q", got, "only this")
		}
	})

	t.Run("array_skips_empty_text", func(t *testing.T) {
		blocks := []any{
			map[string]any{"text": "  "},
			map[string]any{"text": "real"},
		}
		got := extractOpenAIContent(blocks)
		if got != "real" {
			t.Fatalf("got %q, want %q", got, "real")
		}
	})

	t.Run("empty_array", func(t *testing.T) {
		if got := extractOpenAIContent([]any{}); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})

	t.Run("fallback_type", func(t *testing.T) {
		if got := extractOpenAIContent(42); got != "42" {
			t.Fatalf("got %q, want %q", got, "42")
		}
	})
}

func TestNormalizeJSONBytes(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"nil", nil, "{}"},
		{"empty", []byte(""), "{}"},
		{"whitespace_only", []byte("   "), "{}"},
		{"valid_json", []byte(`{"key":"value"}`), `{"key":"value"}`},
		{"valid_json_with_whitespace", []byte(`  {"a":1}  `), `{"a":1}`},
		{"invalid_json", []byte("not json"), "{}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(normalizeJSONBytes(tt.input))
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeJSONString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "{}"},
		{"whitespace_only", "   ", "{}"},
		{"valid_json", `{"key":"value"}`, `{"key":"value"}`},
		{"json_string_passthrough", `"{\"inner\":true}"`, `"{\"inner\":true}"`},
		{"go_hex_escaped_to_json", `"\x7b\"key\":\"val\"\x7d"`, `{"key":"val"}`},
		{"invalid", "not json at all", "{}"},
		{"valid_array", `[1,2,3]`, `[1,2,3]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(normalizeJSONString(tt.input))
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
