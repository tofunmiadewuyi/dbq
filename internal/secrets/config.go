package secrets

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

func configDir() string {
	if os.Getuid() == 0 {
		return "/etc/dbq"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dbq")
}

// NewConfigured opens the explicitly selected standalone store. An omitted
// provider preserves the existing keyring default; errors never select another store.
func NewConfigured() (Store, error) {
	var settings struct {
		Secrets struct {
			Provider string `toml:"provider"`
		} `toml:"secrets"`
	}
	path := filepath.Join(configDir(), "config.toml")
	if _, err := toml.DecodeFile(path, &settings); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read secrets configuration: %w", err)
	}
	// Generated services pin their backend so changing the interactive default
	// cannot accidentally make them read a different store.
	provider := settings.Secrets.Provider
	if override := os.Getenv("DBQ_SECRETS_PROVIDER"); override != "" {
		provider = override
	}
	switch provider {
	case "", "keyring":
		return NewKeyring()
	case "systemd":
		return NewSystemd(filepath.Join(configDir(), "credentials"))
	default:
		return nil, fmt.Errorf("unknown secrets provider %q: choose keyring or systemd; external actors use dbq agent", provider)
	}
}
