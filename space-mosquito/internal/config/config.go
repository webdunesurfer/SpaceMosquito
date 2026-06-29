package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Session  SessionConfig  `yaml:"session"`
	Embedder EmbedderConfig `yaml:"embedder"`
	MCP      MCPConfig      `yaml:"mcp"`
	Cron     CronConfig     `yaml:"cron"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

func (c DatabaseConfig) DSN() string {
	if os.Getenv("DATABASE_URL") != "" {
		return os.Getenv("DATABASE_URL")
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

type StorageConfig struct {
	BasePath string `yaml:"base_path"`
}

type SessionConfig struct {
	EncryptionKey string `yaml:"encryption_key"`
	FilePath      string `yaml:"file_path"`
}

type EmbedderConfig struct {
	Model  string           `yaml:"model"`
	OpenAI *OpenAICConfig  `yaml:"openai"`
	BGE    *BGEConfig      `yaml:"bge"`
}

type OpenAICConfig struct {
	APIKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

type BGEConfig struct {
	ModelPath string `yaml:"model_path"`
}

type MCPConfig struct {
	Port               int    `yaml:"port"`
	Host               string `yaml:"host"`
	Timeout            int    `yaml:"session_timeout"`
	ExposeInternalIDs  bool   `yaml:"expose_internal_ids"`
}

type CronConfig struct {
	FullCrawl   *CronJobConfig `yaml:"full_crawl"`
	Incremental *CronJobConfig `yaml:"incremental"`
}

type CronJobConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Interval   string   `yaml:"interval"`
	Spaces     []string `yaml:"spaces"`
	Detection  string   `yaml:"detection"`
	MaxDuration string  `yaml:"max_duration"`
}

func Load(path string) (*Config, error) {
	expanded := os.ExpandEnv(path)
	if expanded != path {
		p, err := filepath.Abs(expanded)
		if err == nil {
			path = p
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Storage.BasePath == "" {
		cfg.Storage.BasePath = "./saved"
	}
	if cfg.MCP.Port == 0 {
		cfg.MCP.Port = 8081
	}
	if cfg.MCP.Timeout == 0 {
		cfg.MCP.Timeout = 3600
	}
	if cfg.Embedder.Model == "" {
		cfg.Embedder.Model = "nomic-embed-text"
	}

	// Cron defaults
	if cfg.Cron.FullCrawl != nil && cfg.Cron.FullCrawl.Interval == "" {
		cfg.Cron.FullCrawl.Interval = "24h"
	}
	if cfg.Cron.Incremental != nil && cfg.Cron.Incremental.Interval == "" {
		cfg.Cron.Incremental.Interval = "2h"
	}
	if cfg.Cron.Incremental != nil && cfg.Cron.Incremental.Detection == "" {
		cfg.Cron.Incremental.Detection = "dom"
	}
	if cfg.Cron.FullCrawl != nil && cfg.Cron.FullCrawl.MaxDuration == "" {
		cfg.Cron.FullCrawl.MaxDuration = "4h"
	}
	if cfg.Cron.Incremental != nil && cfg.Cron.Incremental.MaxDuration == "" {
		cfg.Cron.Incremental.MaxDuration = "30m"
	}

	return &cfg, nil
}

// ParseCronDuration parses a cron interval string and returns the duration.
func ParseCronDuration(d string) (time.Duration, error) {
	return time.ParseDuration(d)
}

// ParseMaxDuration parses the MaxDuration string and returns a duration with a default fallback.
func (c *CronJobConfig) ParseMaxDuration() (time.Duration, error) {
	if c == nil || c.MaxDuration == "" {
		return 4 * time.Hour, nil
	}
	return time.ParseDuration(c.MaxDuration)
}
