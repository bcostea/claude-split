// Package folders persists a per-folder "last explicitly used split" map so a
// bare launch in a known folder can auto-load the split chosen there before.
package folders

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Store is a map of canonical absolute folder path to split name.
type Store struct {
	path string
	m    map[string]string
}

// DefaultPath returns ${XDG_CONFIG_HOME:-~/.config}/claude-split/folders.json.
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "claude-split", "folders.json")
}

// Load reads the map from path. A missing file yields an empty, saveable store.
func Load(path string) (*Store, error) {
	s := &Store{path: path, m: map[string]string{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if s.m == nil {
		s.m = map[string]string{}
	}
	return s, nil
}

func (s *Store) Get(folder string) (string, bool) {
	v, ok := s.m[folder]
	return v, ok
}

func (s *Store) Set(folder, split string) { s.m[folder] = split }

func (s *Store) Delete(folder string) { delete(s.m, folder) }

func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
