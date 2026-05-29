package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/lite-claw/lite-claw/internal/llm"
	"github.com/lite-claw/lite-claw/internal/store"
)

// Tool executes a named capability.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Run(args map[string]any) (string, error)
}

// Registry holds agent tools.
type Registry struct {
	workspace string
	mu        sync.RWMutex
	memory    map[string]string
	memStore  store.MemoryStore
	ctx       context.Context
	sessionID string
	tools     []Tool
}

func NewRegistry(workspace string, mem store.MemoryStore) *Registry {
	r := &Registry{
		workspace: workspace,
		memory:    make(map[string]string),
		memStore:  mem,
	}
	r.tools = []Tool{
		&shellTool{r: r},
		&readFileTool{r: r},
		&writeFileTool{r: r},
		&listDirTool{r: r},
		&rememberTool{r: r},
		&recallTool{r: r},
	}
	return r
}

func (r *Registry) SetSession(ctx context.Context, sessionID string) {
	r.ctx = ctx
	r.sessionID = sessionID
}

func (r *Registry) Definitions() []llm.ToolDef {
	defs := make([]llm.ToolDef, len(r.tools))
	for i, t := range r.tools {
		defs[i] = llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		}
	}
	return defs
}

func (r *Registry) Run(name string, argsJSON string) (string, error) {
	var args map[string]any
	if argsJSON != "" && argsJSON != "{}" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid tool args: %w", err)
		}
	}
	for _, t := range r.tools {
		if t.Name() == name {
			return t.Run(args)
		}
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func (r *Registry) resolvePath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path required")
	}
	base := r.workspace
	if !filepath.IsAbs(base) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		base = filepath.Join(wd, base)
	}
	clean := filepath.Clean(p)
	if filepath.IsAbs(clean) {
		if !strings.HasPrefix(clean, base) {
			return "", fmt.Errorf("path outside workspace")
		}
		return clean, nil
	}
	full := filepath.Join(base, clean)
	if !strings.HasPrefix(full, base) {
		return "", fmt.Errorf("path outside workspace")
	}
	return full, nil
}

type shellTool struct{ r *Registry }

func (t *shellTool) Name() string { return "shell" }
func (t *shellTool) Description() string {
	return "Run a shell command in the workspace directory. Returns combined stdout/stderr."
}
func (t *shellTool) Parameters() map[string]any {
	return objSchema(map[string]any{
		"command": prop("string", "Shell command to run"),
	}, []string{"command"})
}
func (t *shellTool) Run(args map[string]any) (string, error) {
	cmdStr, _ := args["command"].(string)
	if strings.TrimSpace(cmdStr) == "" {
		return "", fmt.Errorf("command required")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	cmd.Dir = t.r.workspace
	if !filepath.IsAbs(cmd.Dir) {
		wd, _ := os.Getwd()
		cmd.Dir = filepath.Join(wd, cmd.Dir)
	}
	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		result += "\n[exit error: " + err.Error() + "]"
	}
	return strings.TrimSpace(result), nil
}

type readFileTool struct{ r *Registry }

func (t *readFileTool) Name() string { return "read_file" }
func (t *readFileTool) Description() string {
	return "Read a text file relative to the workspace."
}
func (t *readFileTool) Parameters() map[string]any {
	return objSchema(map[string]any{
		"path": prop("string", "Relative file path"),
	}, []string{"path"})
}
func (t *readFileTool) Run(args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	full, err := t.r.resolvePath(p)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	const max = 64 * 1024
	if len(data) > max {
		return string(data[:max]) + "\n...[truncated]", nil
	}
	return string(data), nil
}

type writeFileTool struct{ r *Registry }

func (t *writeFileTool) Name() string { return "write_file" }
func (t *writeFileTool) Description() string {
	return "Write content to a file relative to the workspace (creates parent dirs)."
}
func (t *writeFileTool) Parameters() map[string]any {
	return objSchema(map[string]any{
		"path":    prop("string", "Relative file path"),
		"content": prop("string", "File content"),
	}, []string{"path", "content"})
}
func (t *writeFileTool) Run(args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	content, _ := args["content"].(string)
	full, err := t.r.resolvePath(p)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), p), nil
}

type listDirTool struct{ r *Registry }

func (t *listDirTool) Name() string { return "list_dir" }
func (t *listDirTool) Description() string {
	return "List files and directories in a workspace path."
}
func (t *listDirTool) Parameters() map[string]any {
	return objSchema(map[string]any{
		"path": prop("string", "Relative directory path (default .)"),
	}, nil)
}
func (t *listDirTool) Run(args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		p = "."
	}
	full, err := t.r.resolvePath(p)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			b.WriteString("[dir]  ")
		} else {
			b.WriteString("[file] ")
		}
		b.WriteString(e.Name())
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String()), nil
}

type rememberTool struct{ r *Registry }

func (t *rememberTool) Name() string { return "remember" }
func (t *rememberTool) Description() string {
	return "Store a key-value fact in session memory for later recall."
}
func (t *rememberTool) Parameters() map[string]any {
	return objSchema(map[string]any{
		"key":   prop("string", "Memory key"),
		"value": prop("string", "Value to store"),
	}, []string{"key", "value"})
}
func (t *rememberTool) Run(args map[string]any) (string, error) {
	key, _ := args["key"].(string)
	val, _ := args["value"].(string)
	if key == "" {
		return "", fmt.Errorf("key required")
	}
	if t.r.memStore != nil && t.r.ctx != nil {
		if err := t.r.memStore.Set(t.r.ctx, t.r.sessionID, key, val); err != nil {
			return "", err
		}
		return "ok", nil
	}
	t.r.mu.Lock()
	t.r.memory[key] = val
	t.r.mu.Unlock()
	return "ok", nil
}

type recallTool struct{ r *Registry }

func (t *recallTool) Name() string { return "recall" }
func (t *recallTool) Description() string {
	return "Recall a stored memory by key, or list all keys if key is empty."
}
func (t *recallTool) Parameters() map[string]any {
	return objSchema(map[string]any{
		"key": prop("string", "Memory key (optional)"),
	}, nil)
}
func (t *recallTool) Run(args map[string]any) (string, error) {
	key, _ := args["key"].(string)
	if t.r.memStore != nil && t.r.ctx != nil {
		if key == "" {
			keys, err := t.r.memStore.ListKeys(t.r.ctx, t.r.sessionID)
			if err != nil {
				return "", err
			}
			if len(keys) == 0 {
				return "(no memories)", nil
			}
			return strings.Join(keys, ", "), nil
		}
		return t.r.memStore.Get(t.r.ctx, t.r.sessionID, key)
	}
	t.r.mu.RLock()
	defer t.r.mu.RUnlock()
	if key == "" {
		keys := make([]string, 0, len(t.r.memory))
		for k := range t.r.memory {
			keys = append(keys, k)
		}
		if len(keys) == 0 {
			return "(no memories)", nil
		}
		return strings.Join(keys, ", "), nil
	}
	v, ok := t.r.memory[key]
	if !ok {
		return "", fmt.Errorf("no memory for key %q", key)
	}
	return v, nil
}

func objSchema(props map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func prop(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}
