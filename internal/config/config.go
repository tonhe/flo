package config

import (
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Theme           string        `toml:"theme"`
	DefaultIdentity string        `toml:"default_identity"`
	PollInterval    time.Duration `toml:"-"`
	PollIntervalStr string        `toml:"poll_interval"`
	MaxHistory      int           `toml:"max_history"`
}

func DefaultConfig() *Config {
	return &Config{
		Theme:           "solarized-dark",
		DefaultIdentity: "",
		PollInterval:    10 * time.Second,
		PollIntervalStr: "10s",
		MaxHistory:      360,
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
	if cfg.PollIntervalStr != "" {
		d, err := time.ParseDuration(cfg.PollIntervalStr)
		if err == nil {
			cfg.PollInterval = d
		}
	}
	return cfg, nil
}

func SaveConfig(cfg *Config, path string) error {
	cfg.PollIntervalStr = cfg.PollInterval.String()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
