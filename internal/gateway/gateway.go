package gateway

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lite-claw/lite-claw/internal/agent"
	"github.com/lite-claw/lite-claw/internal/channel/whatsapp"
	"github.com/lite-claw/lite-claw/internal/config"
	"github.com/lite-claw/lite-claw/internal/llm"
	"github.com/lite-claw/lite-claw/internal/store"
	"github.com/lite-claw/lite-claw/internal/supabase"
)

// Gateway wires channels to the agent runtime.
type Gateway struct {
	cfg      *config.Config
	provider llm.Provider
	sessions store.Store
	msgLog   store.MessageLogger
	agent    *agent.Agent
}

func New(cfg *config.Config) (*Gateway, error) {
	if err := os.MkdirAll(cfg.Gateway.DataDir, 0o755); err != nil {
		return nil, err
	}
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	if o, ok := provider.(*llm.OllamaProvider); ok {
		if err := o.Ping(context.Background()); err != nil {
			log.Printf("warning: ollama not reachable at configured URL: %v", err)
			log.Printf("start Ollama (`ollama serve`) and pull a model (`ollama pull %s`)", cfg.Agent.Model)
		}
	}
	sessions, err := store.New(cfg)
	if err != nil {
		return nil, err
	}
	msgLog, err := store.NewMessageLogger(cfg)
	if err != nil {
		return nil, err
	}
	memStore, err := store.NewMemoryStore(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Database.Driver == "supabase" {
		log.Printf("database: supabase enabled")
	}
	return &Gateway{
		cfg:      cfg,
		provider: provider,
		sessions: sessions,
		msgLog:   msgLog,
		agent:    agent.New(cfg, provider, memStore),
	}, nil
}

// PingDatabase checks Supabase connectivity when configured.
func PingDatabase(cfg *config.Config) error {
	if cfg.Database.Driver != "supabase" {
		return fmt.Errorf("database driver is %q, not supabase", cfg.Database.Driver)
	}
	client, err := supabase.NewClient(supabase.Config{
		URL:        cfg.Database.Supabase.URL,
		AnonKey:    cfg.Database.Supabase.AnonKey,
		ServiceKey: cfg.Database.Supabase.ServiceKey,
	})
	if err != nil {
		return err
	}
	return client.Ping()
}

// RunAgentOnce runs a single agent turn from CLI (no channel).
func (g *Gateway) RunAgentOnce(ctx context.Context, sessionID, message string) (string, error) {
	history, _ := g.sessions.Load(ctx, sessionID)
	reply, updated, err := g.agent.Run(ctx, sessionID, history, message)
	if err != nil {
		return "", err
	}
	toSave := store.FilterPersistable(updated)
	if err := g.sessions.Save(ctx, sessionID, toSave); err != nil {
		return "", err
	}
	return reply, nil
}

// Run starts enabled channels until interrupted.
func (g *Gateway) Run(ctx context.Context) error {
	if !g.cfg.Channels.WhatsApp.Enabled {
		return fmt.Errorf("no channels enabled; enable channels.whatsapp in config")
	}

	wa := whatsapp.New(g.cfg.Channels.WhatsApp, g.cfg.Gateway.DataDir, g.handleWhatsApp)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("lite-claw gateway starting (provider=%s model=%s db=%s)", g.cfg.Agent.Provider, g.cfg.Agent.Model, g.cfg.Database.Driver)
	return wa.Run(ctx)
}

// LoginWhatsApp pairs WhatsApp via QR.
func (g *Gateway) LoginWhatsApp(ctx context.Context) error {
	wa := whatsapp.New(g.cfg.Channels.WhatsApp, g.cfg.Gateway.DataDir, nil)
	return wa.Login(ctx)
}

func (g *Gateway) handleWhatsApp(ctx context.Context, sessionID, sender, text string) (string, error) {
	log.Printf("whatsapp from %s: %s", sender, truncate(text, 80))
	g.logMessage(ctx, sessionID, "whatsapp", "in", sender, text)

	history, _ := g.sessions.Load(ctx, sessionID)
	reply, updated, err := g.agent.Run(ctx, sessionID, history, text)
	if err != nil {
		return "", err
	}
	toSave := store.FilterPersistable(updated)
	if err := g.sessions.Save(ctx, sessionID, toSave); err != nil {
		return "", err
	}

	g.logMessage(ctx, sessionID, "whatsapp", "out", sender, reply)
	log.Printf("whatsapp reply to %s: %s", sender, truncate(reply, 80))
	return reply, nil
}

func (g *Gateway) logMessage(ctx context.Context, sessionID, channel, direction, sender, content string) {
	if g.msgLog == nil {
		return
	}
	_ = g.msgLog.Log(ctx, store.MessageLogEntry{
		SessionID: sessionID,
		Channel:   channel,
		Direction: direction,
		Sender:    sender,
		Content:   content,
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
