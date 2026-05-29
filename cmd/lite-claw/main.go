package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/lite-claw/lite-claw/internal/config"
	"github.com/lite-claw/lite-claw/internal/gateway"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cfgPath := os.Getenv("LITE_CLAW_CONFIG")

	switch os.Args[1] {
	case "gateway":
		fs := flag.NewFlagSet("gateway", flag.ExitOnError)
		configFlag := fs.String("config", "", "path to config.json")
		_ = fs.Parse(os.Args[2:])
		runGateway(resolveConfig(cfgPath, *configFlag))

	case "agent":
		fs := flag.NewFlagSet("agent", flag.ExitOnError)
		configFlag := fs.String("config", "", "path to config.json")
		message := fs.String("message", "", "user message")
		_ = fs.Parse(os.Args[2:])
		if *message == "" {
			fatal(fmt.Errorf("usage: lite-claw agent --message \"your message\""))
		}
		runAgent(resolveConfig(cfgPath, *configFlag), *message)

	case "channels":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: lite-claw channels login [--channel whatsapp]"))
		}
		fs := flag.NewFlagSet("channels", flag.ExitOnError)
		configFlag := fs.String("config", "", "path to config.json")
		channel := fs.String("channel", "whatsapp", "channel name")
		_ = fs.Parse(os.Args[3:])
		if os.Args[2] != "login" {
			fatal(fmt.Errorf("usage: lite-claw channels login"))
		}
		if *channel != "whatsapp" {
			fatal(fmt.Errorf("only whatsapp is supported in MVP"))
		}
		runChannelsLogin(resolveConfig(cfgPath, *configFlag))

	case "config":
		if len(os.Args) < 3 || os.Args[2] != "init" {
			fatal(fmt.Errorf("usage: lite-claw config init"))
		}
		runConfigInit(cfgPath)

	case "db":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: lite-claw db ping"))
		}
		fs := flag.NewFlagSet("db", flag.ExitOnError)
		configFlag := fs.String("config", "", "path to config.json")
		_ = fs.Parse(os.Args[3:])
		switch os.Args[2] {
		case "ping":
			runDBPing(resolveConfig(cfgPath, *configFlag))
		default:
			fatal(fmt.Errorf("usage: lite-claw db ping"))
		}

	case "help", "-h", "--help":
		usage()

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func resolveConfig(env, flag string) string {
	if flag != "" {
		return flag
	}
	return env
}

func loadConfig(path string) (*config.Config, string, error) {
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, err := config.Load(path)
	return cfg, path, err
}

func runGateway(path string) {
	cfg, used, err := loadConfig(path)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("using config %s\n", used)
	gw, err := gateway.New(cfg)
	if err != nil {
		fatal(err)
	}
	if err := gw.Run(context.Background()); err != nil && err != context.Canceled {
		fatal(err)
	}
}

func runAgent(path, message string) {
	cfg, used, err := loadConfig(path)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("using config %s\n", used)
	gw, err := gateway.New(cfg)
	if err != nil {
		fatal(err)
	}
	reply, err := gw.RunAgentOnce(context.Background(), "cli", message)
	if err != nil {
		fatal(err)
	}
	fmt.Println(reply)
}

func runChannelsLogin(path string) {
	cfg, used, err := loadConfig(path)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("using config %s\n", used)
	gw, err := gateway.New(cfg)
	if err != nil {
		fatal(err)
	}
	if err := gw.LoginWhatsApp(context.Background()); err != nil {
		fatal(err)
	}
}

func runConfigInit(explicit string) {
	path := explicit
	if path == "" {
		path = config.DefaultPath()
	}
	cfg := config.Default()
	if err := cfg.Save(path); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote default config to %s\n", path)
}

func runDBPing(path string) {
	cfg, used, err := loadConfig(path)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("using config %s\n", used)
	if err := gateway.PingDatabase(cfg); err != nil {
		fatal(err)
	}
	fmt.Println("supabase: connected (sessions table reachable)")
}

func usage() {
	fmt.Print(`lite-claw — lightweight OpenClaw-style agent gateway (Go)

Usage:
  lite-claw gateway [--config path]     Start gateway (WhatsApp + agent)
  lite-claw channels login              Pair WhatsApp via QR code
  lite-claw agent --message "…"         Test agent locally (CLI session)
  lite-claw config init                 Write default config
  lite-claw db ping                     Test Supabase connection

Environment:
  LITE_CLAW_CONFIG              Config file path
  OPENAI_API_KEY                For openai provider
  ANTHROPIC_API_KEY             For anthropic-compatible proxies
  SUPABASE_URL                  Supabase project URL
  SUPABASE_SERVICE_ROLE_KEY     Supabase service key (gateway)
  SUPABASE_ANON_KEY             Supabase anon key (fallback)

Config default: ~/.lite-claw/config.json
`)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
