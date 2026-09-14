package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shurco/mycart/pkg/fsutil"
)

// fileConfig is the on-disk shape of lc_base/config.json. It is written by the
// installer and read on every start.
type fileConfig struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

// readConfigFile loads the configuration written by a previous install. A
// missing file is not an error: it means the installation is using the built-in
// default.
func readConfigFile(path string) (Config, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, false, nil
		}
		return Config{}, false, fmt.Errorf("read database config %s: %w", path, err)
	}

	var fc fileConfig
	if err := json.Unmarshal(raw, &fc); err != nil {
		return Config{}, false, fmt.Errorf("parse database config %s: %w", path, err)
	}
	if fc.Driver == "" {
		return Config{}, false, fmt.Errorf("database config %s: driver is required", path)
	}

	return Config{Driver: fc.Driver, DSN: fc.DSN, Source: SourceFile}, true, nil
}

// WriteConfig persists the chosen database so the next start connects to the
// same place. The file holds a password, hence mode 0600.
func WriteConfig(c Config) error {
	return WriteConfigFile(c, ConfigPath)
}

// WriteConfigFile is WriteConfig with an explicit path.
func WriteConfigFile(c Config, path string) error {
	raw, err := json.MarshalIndent(fileConfig{Driver: c.Driver, DSN: c.DSN}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode database config: %w", err)
	}

	if err := fsutil.MkDirs(0o775, filepath.Dir(path)); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("write database config %s: %w", path, err)
	}
	return nil
}

// RemoveConfig deletes the persisted configuration, returning to the default.
func RemoveConfig() error {
	err := os.Remove(ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove database config %s: %w", ConfigPath, err)
	}
	return nil
}
