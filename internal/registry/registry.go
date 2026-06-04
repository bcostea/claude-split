package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Registry is the persisted set of splits and the configured auto-default.
// "default" is a reserved name meaning the home profile; it is never listed
// in Splits.
type Registry struct {
	Splits  []string `json:"splits"`
	Default string   `json:"default"`

	path string
}

// Load reads registry.json from baseDir. A missing file yields an empty,
// saveable registry (no error).
func Load(baseDir string) (*Registry, error) {
	r := &Registry{path: filepath.Join(baseDir, "registry.json")}
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", r.path, err)
	}
	return r, nil
}

func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}

func (r *Registry) Has(name string) bool {
	for _, s := range r.Splits {
		if s == name {
			return true
		}
	}
	return false
}

func (r *Registry) Add(name string) error {
	if r.Has(name) {
		return fmt.Errorf("split %q already exists", name)
	}
	r.Splits = append(r.Splits, name)
	return nil
}

func (r *Registry) Remove(name string) error {
	if name == "default" {
		return errors.New("cannot remove the default (home) profile")
	}
	if !r.Has(name) {
		return fmt.Errorf("split %q does not exist", name)
	}
	out := make([]string, 0, len(r.Splits)-1)
	for _, s := range r.Splits {
		if s != name {
			out = append(out, s)
		}
	}
	r.Splits = out
	if r.Default == name {
		r.Default = ""
	}
	return nil
}

func (r *Registry) SetDefault(name string) error {
	if name != "default" && !r.Has(name) {
		return fmt.Errorf("split %q does not exist", name)
	}
	r.Default = name
	return nil
}
