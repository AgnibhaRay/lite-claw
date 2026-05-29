package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/lite-claw/lite-claw/internal/config"
	"github.com/lite-claw/lite-claw/internal/llm"
	"github.com/lite-claw/lite-claw/internal/store"
)

// Agent runs the tool loop against an LLM provider.
type Agent struct {
	cfg      *config.Config
	provider llm.Provider
	tools    *Registry
}

func New(cfg *config.Config, provider llm.Provider, mem store.MemoryStore) *Agent {
	ws := cfg.Agent.Workspace
	if ws == "" {
		ws = "."
	}
	return &Agent{
		cfg:      cfg,
		provider: provider,
		tools:    NewRegistry(ws, mem),
	}
}

// Run processes a user message with optional prior history and returns the assistant reply.
func (a *Agent) Run(ctx context.Context, sessionID string, history []llm.Message, userMsg string) (string, []llm.Message, error) {
	a.tools.SetSession(ctx, sessionID)
	messages := make([]llm.Message, 0, len(history)+2)
	if a.cfg.Agent.System != "" {
		messages = append(messages, llm.Message{Role: llm.RoleSystem, Content: a.cfg.Agent.System})
	}
	messages = append(messages, history...)
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: userMsg})

	toolDefs := a.tools.Definitions()
	maxTurns := a.cfg.Agent.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 12
	}

	for turn := 0; turn < maxTurns; turn++ {
		resp, err := a.provider.Chat(ctx, llm.ChatRequest{
			Model:    a.cfg.Agent.Model,
			Messages: messages,
			Tools:    toolDefs,
		})
		if err != nil {
			return "", messages, err
		}

		if len(resp.ToolCalls) == 0 {
			reply := strings.TrimSpace(resp.Content)
			if reply == "" {
				reply = "(no response)"
			}
			messages = append(messages, llm.Message{Role: llm.RoleAssistant, Content: reply})
			return reply, messages, nil
		}

		// Assistant message with tool calls (content may be empty)
		messages = append(messages, llm.Message{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		for _, tc := range resp.ToolCalls {
			result, runErr := a.tools.Run(tc.Name, tc.Arguments)
			if runErr != nil {
				result = "error: " + runErr.Error()
			}
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
			})
		}
	}

	return "", messages, fmt.Errorf("max tool turns (%d) exceeded", maxTurns)
}
