# lite-claw

A **lightweight**, **fast** OpenClaw-style agent gateway written in **Go**. Connect messaging apps to a local or cloud LLM with tool use.

## Features (MVP)

- **Ollama** first-class integration (local models)
- **OpenAI-compatible** providers (OpenAI, Groq, Together, local proxies)
- **WhatsApp** via [whatsmeow](https://github.com/tulir/whatsmeow) (QR pairing)
- **Agent tools**: `shell`, `read_file`, `write_file`, `list_dir`, `remember`, `recall`
- **Per-chat sessions** persisted on disk
- Single binary, minimal dependencies

## Prerequisites

1. [Go 1.22+](https://go.dev/dl/)
2. [Ollama](https://ollama.com/) running locally (for default setup)
3. A tool-capable model, e.g. `ollama pull llama3.2`

```bash
ollama serve
ollama pull llama3.2
```

## Quick start

```bash
# Build (pure Go — no C compiler needed on Windows)
set CGO_ENABLED=0
go build -o lite-claw.exe ./cmd/lite-claw
```

# Create config (~/.lite-claw/config.json)
./lite-claw.exe config init

# Test agent without WhatsApp
./lite-claw.exe agent --message "List files in my workspace"

# Pair WhatsApp (scan QR in terminal)
./lite-claw.exe channels login

# Run gateway (WhatsApp → agent → reply)
./lite-claw.exe gateway
```

## Configuration

Config path: `~/.lite-claw/config.json` or set `LITE_CLAW_CONFIG`.

| Field | Description |
|-------|-------------|
| `agent.provider` | `ollama`, `openai`, or custom OpenAI-compatible name |
| `agent.model` | Model id (e.g. `llama3.2`, `gpt-4o-mini`) |
| `agent.workspace` | Sandbox for file/shell tools |
| `channels.whatsapp.allowFrom` | E.164 numbers or `*` |
| `channels.whatsapp.selfChat` | Reply to messages you send yourself |
| `providers.ollama.baseURL` | Default `http://127.0.0.1:11434` |

See [config.example.json](config.example.json).

### OpenAI / cloud

```json
{
  "agent": {
    "provider": "openai",
    "model": "gpt-4o-mini"
  },
  "providers": {
    "openai": {
      "apiKey": "sk-..."
    }
  }
}
```

Or set `OPENAI_API_KEY` in the environment.

## Commands

| Command | Purpose |
|---------|---------|
| `lite-claw gateway` | Start WhatsApp listener + agent |
| `lite-claw channels login` | Link WhatsApp device (QR) |
| `lite-claw agent --message "…"` | CLI test without WhatsApp |
| `lite-claw config init` | Write default config |

## Supabase database

lite-claw can persist sessions, memories, contacts, and message logs to Supabase.

### 1. Run the migration

In Supabase **SQL Editor**, run:

`supabase/migrations/001_initial.sql`

### 2. Configure credentials

Set in config or environment:

```json
"database": {
  "driver": "supabase",
  "supabase": {
    "url": "https://YOUR_PROJECT.supabase.co",
    "serviceKey": "YOUR_SERVICE_ROLE_KEY"
  }
}
```

Or via env:

```powershell
$env:SUPABASE_URL="https://YOUR_PROJECT.supabase.co"
$env:SUPABASE_SERVICE_ROLE_KEY="your-service-role-key"
```

### 3. Test connection

```powershell
.\lite-claw.exe db ping --config your-config.json
```

### SDK layout

| Package | Purpose |
|---------|---------|
| `internal/supabase` | Client + repositories (sessions, messages, memories, contacts, logs) |
| `internal/store` | Pluggable store (`file` or `supabase`) |
| `supabase/migrations/` | Postgres schema |

When `database.driver` is `file` (default), sessions save to `~/.lite-claw/sessions/`. When `supabase`, everything goes to Postgres.

---

## Architecture

```
WhatsApp (whatsmeow) ──► Gateway ──► Agent (tool loop) ──► LLM (Ollama / OpenAI-compat)
                              │
                              └── Session store (JSON per chat)
```

## Security notes

- Restrict `channels.whatsapp.allowFrom` in production (avoid `*`).
- `shell` runs in `agent.workspace` — point it at a dedicated directory.
- WhatsApp uses an unofficial API; use a separate number if possible.

## Roadmap

- Telegram, Discord, Slack channels
- HTTP gateway API + WebChat
- Cron / scheduled tasks
- Browser tool
- Anthropic native API

## License

MIT
