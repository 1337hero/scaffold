package signal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const defaultTimeout = 10 * time.Second
const maxSSELineSize = 4 * 1024 * 1024

type Client struct {
	baseURL    string
	account    string
	httpClient *http.Client
}

func NewClient(baseURL, account string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		account: account,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

type rpcRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
	ID      string                 `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	ID      string          `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Check verifies signal-cli daemon is reachable.
func (c *Client) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/check", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("signal-cli not reachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("signal-cli check failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Send sends a message to a recipient phone number.
func (c *Client) Send(ctx context.Context, recipient, message string) error {
	params := map[string]interface{}{
		"recipients": []string{recipient},
		"message":    message,
	}
	if c.account != "" {
		params["account"] = c.account
	}
	return c.rpc(ctx, "send", params, nil)
}

func (c *Client) rpc(ctx context.Context, method string, params map[string]interface{}, out interface{}) error {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      uuid.New().String(),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/rpc", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 {
		return nil
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("invalid RPC response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out != nil && rpcResp.Result != nil {
		return json.Unmarshal(rpcResp.Result, out)
	}
	return nil
}

// SSEEvent is a parsed Server-Sent Event.
type SSEEvent struct {
	Event string
	Data  string
	ID    string
}

// InboundMessage is a Signal message extracted from an SSE envelope event.
type InboundMessage struct {
	Sender  string
	Message string
	Raw     json.RawMessage
}

// StreamEvents connects to signal-cli SSE and calls onEvent for each event.
// Blocks until ctx is cancelled or connection drops.
func (c *Client) StreamEvents(ctx context.Context, onEvent func(SSEEvent)) error {
	streamURL, err := url.Parse(c.baseURL + "/api/v1/events")
	if err != nil {
		return err
	}
	if c.account != "" {
		q := streamURL.Query()
		q.Set("account", c.account)
		streamURL.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", streamURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	// SSE needs a client without a short timeout
	sseClient := &http.Client{}
	resp, err := sseClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !isOK(resp.StatusCode) {
		return fmt.Errorf("SSE stream failed: HTTP %d", resp.StatusCode)
	}

	return parseSSE(resp.Body, onEvent)
}

// ParseInbound extracts an InboundMessage from an SSE envelope event.
// Returns nil if the event is not an inbound message.
func ParseInbound(event SSEEvent) *InboundMessage {
	if event.Event != "receive" && event.Event != "" {
		// signal-cli uses empty event name for message envelopes in some versions
		// Try to parse regardless
	}
	if event.Data == "" {
		return nil
	}

	var envelope struct {
		Account  string `json:"account"`
		Envelope struct {
			Source      string `json:"source"`
			DataMessage *struct {
				Message string `json:"message"`
			} `json:"dataMessage"`
		} `json:"envelope"`
	}

	if err := json.Unmarshal([]byte(event.Data), &envelope); err != nil {
		return nil
	}
	if envelope.Envelope.DataMessage == nil {
		return nil
	}

	return &InboundMessage{
		Sender:  envelope.Envelope.Source,
		Message: envelope.Envelope.DataMessage.Message,
		Raw:     json.RawMessage(event.Data),
	}
}

func parseSSE(r io.Reader, onEvent func(SSEEvent)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxSSELineSize)
	current := SSEEvent{}

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Dispatch event
			if current.Data != "" || current.Event != "" || current.ID != "" {
				onEvent(current)
				current = SSEEvent{}
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // comment
		}

		field, value, _ := strings.Cut(line, ":")
		if strings.HasPrefix(value, " ") {
			value = value[1:]
		}

		switch field {
		case "event":
			current.Event = value
		case "data":
			if current.Data != "" {
				current.Data += "\n" + value
			} else {
				current.Data = value
			}
		case "id":
			current.ID = value
		}
	}
	return scanner.Err()
}

func isOK(status int) bool {
	return status >= 200 && status < 300
}
