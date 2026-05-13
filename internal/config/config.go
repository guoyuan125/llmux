package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
	Auth     AuthConfig     `yaml:"auth"`
	Metrics  MetricsConfig  `yaml:"metrics"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

func (s ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	Type string `yaml:"type"` // sqlite
	Path string `yaml:"path"`
}

type LogConfig struct {
	Level string `yaml:"level"` // debug, info, warn, error
}

type AuthConfig struct {
	JWTSecret   string `yaml:"jwt_secret"`
	AdminUser   string `yaml:"admin_user"`
	AdminPass   string `yaml:"admin_pass"`
	KeyPrefix   string `yaml:"key_prefix"` // API key prefix, e.g. "sk-llmux-"
}

type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Write default config and use it
			if writeErr := writeDefault(path, cfg); writeErr != nil {
				return nil, fmt.Errorf("failed to write default config: %w", writeErr)
			}
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: DatabaseConfig{
			Type: "sqlite",
			Path: "data/data.db",
		},
		Log: LogConfig{
			Level: "info",
		},
		Auth: AuthConfig{
			AdminUser: "admin",
			AdminPass: "admin",
			KeyPrefix: "sk-llmux-",
		},
		Metrics: MetricsConfig{
			Enabled: true,
		},
	}
}

func writeDefault(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
