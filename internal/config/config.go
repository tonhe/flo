package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Theme           string `toml:"theme"`
	DefaultIdentity string `toml:"default_identity"`
	MaxHistory      int    `toml:"max_history"`
	TimeFormat      string `toml:"time_format"`
}

func DefaultConfig() *Config {
	return &Config{
		Theme:           "solarized-dark",
		DefaultIdentity: "",
		MaxHistory:      360,
		TimeFormat:      "relative",
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func SaveConfig(cfg *Config, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
