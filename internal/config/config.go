// Package config loads yaktui's user config file, currently used to
// enable/disable/define addon resources on top of internal/addons.Builtins.
package config

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/vmsilvamolina/yaktui/internal/addons"
)

// File is the on-disk shape of the yaktui config file.
type File struct {
	Addons []addons.Definition `json:"addons"`
}

// DefaultPath returns the config file path: $XDG_CONFIG_HOME/yaktui/config.yaml
// if set, otherwise ~/.config/yaktui/config.yaml.
func DefaultPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "yaktui", "config.yaml"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "yaktui", "config.yaml"), nil
}

// LoadAddonDefinitions returns addons.Builtins merged with any overrides or
// additions found in the config file. A missing config file is not an error.
func LoadAddonDefinitions() ([]addons.Definition, error) {
	defs := append([]addons.Definition{}, addons.Builtins...)

	path, err := DefaultPath()
	if err != nil {
		return defs, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defs, nil
	}
	if err != nil {
		return defs, err
	}

	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return defs, err
	}

	return addons.Merge(defs, f.Addons), nil
}
