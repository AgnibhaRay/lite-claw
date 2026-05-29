package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaProvider calls Ollama's REST API directly (no heavy ollama/ollama module).
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllama(baseURL, defaultModel string) (*OllamaProvider, error) {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return &OllamaProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   defaultModel,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

func (p *OllamaProvider) Name() string { return "ollama" }

type ollamaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatReq struct {
	Model    string       `json:"model"`
	Messages []ollamaMsg  `json:"messages"`
	Tools    []ollamaTool `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

type ollamaToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

type ollamaChatResp struct {
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (p *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	msgs := make([]ollamaMsg, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := string(m.Role)
		if m.Role == RoleTool {
			role = "tool"
		}
		msgs = append(msgs, ollamaMsg{Role: role, Content: m.Content})
	}

	tools := make([]ollamaTool, 0, len(req.Tools))
	for _, t := range req.Tools {
		var ot ollamaTool
		ot.Type = "function"
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = t.Parameters
		tools = append(tools, ot)
	}

	body, _ := json.Marshal(ollamaChatReq{
		Model:    model,
		Messages: msgs,
		Tools:    tools,
		Stream:   false,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama %s: %s", resp.Status, string(raw))
	}

	var parsed ollamaChatResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("ollama parse: %w", err)
	}

	out := &ChatResponse{Content: parsed.Message.Content}
	for _, tc := range parsed.Message.ToolCalls {
		args, _ := json.Marshal(tc.Function.Arguments)
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: string(args),
		})
	}
	out.Stop = len(out.ToolCalls) == 0 && strings.TrimSpace(out.Content) != ""
	return out, nil
}

// Ping checks Ollama is reachable.
func (p *OllamaProvider) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama ping: %s", string(raw))
	}
	return nil
}
