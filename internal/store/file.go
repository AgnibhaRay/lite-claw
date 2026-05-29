package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/lite-claw/lite-claw/internal/llm"
)

// FileStore keeps per-session conversation history on disk.
type FileStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileStore(dataDir string) (*FileStore, error) {
	dir := filepath.Join(dataDir, "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir}, nil
}

func (s *FileStore) path(id string) string {
	safe := filepath.Base(id)
	return filepath.Join(s.dir, safe+".json")
}

func (s *FileStore) Load(_ context.Context, id string) ([]llm.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var msgs []llm.Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, err
	}
	return TrimHistory(msgs, MaxHistory), nil
}

func (s *FileStore) Save(_ context.Context, id string, msgs []llm.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(TrimHistory(msgs, MaxHistory), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(id), data, 0o644)
}
