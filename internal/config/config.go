package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zawachte/huddle/internal/model"
	"sigs.k8s.io/yaml"
)

func Load(path string) (model.Config, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.Config{}, nil, err
	}
	var cfg model.Config
	if err := yaml.UnmarshalStrict(raw, &cfg); err != nil {
		return model.Config{}, nil, fmt.Errorf("decode config: %w", err)
	}
	if err := validate(&cfg, filepath.Dir(path)); err != nil {
		return model.Config{}, nil, err
	}
	return cfg, raw, nil
}

func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validate(cfg *model.Config, base string) error {
	seen := map[string]bool{}
	for i := range cfg.Resources {
		r := &cfg.Resources[i]
		if r.ID == "" || seen[r.ID] {
			return fmt.Errorf("resource %d has an empty or duplicate id %q", i+1, r.ID)
		}
		seen[r.ID] = true
		switch r.Type {
		case "file":
			if r.Path == "" || (r.Source == "" && r.Content == nil) || (r.Source != "" && r.Content != nil) {
				return fmt.Errorf("file %q requires path and exactly one of source or content", r.ID)
			}
			if r.Source != "" && !filepath.IsAbs(r.Source) {
				r.Source = filepath.Join(base, r.Source)
			}
		case "systemd":
			if r.Name == "" {
				return fmt.Errorf("systemd resource %q requires name", r.ID)
			}
		default:
			return fmt.Errorf("resource %q has unsupported type %q", r.ID, r.Type)
		}
	}
	for _, r := range cfg.Resources {
		for _, dep := range append(append([]string{}, r.Requires...), append(r.ReloadWhenChanged, r.RestartWhenChanged...)...) {
			if !seen[dep] {
				return fmt.Errorf("resource %q references unknown resource %q", r.ID, dep)
			}
		}
	}
	return order(cfg)
}

func order(cfg *model.Config) error {
	resources := make(map[string]model.Resource, len(cfg.Resources))
	for _, r := range cfg.Resources {
		resources[r.ID] = r
	}
	state := map[string]uint8{}
	ordered := make([]model.Resource, 0, len(cfg.Resources))
	var visit func(string) error
	visit = func(id string) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("dependency cycle involving resource %q", id)
		case 2:
			return nil
		}
		state[id] = 1
		r := resources[id]
		deps := append(append([]string{}, r.Requires...), append(r.ReloadWhenChanged, r.RestartWhenChanged...)...)
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[id] = 2
		ordered = append(ordered, r)
		return nil
	}
	for _, r := range cfg.Resources {
		if err := visit(r.ID); err != nil {
			return err
		}
	}
	cfg.Resources = ordered
	return nil
}
