package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/vkh/spacemosquito/pkg/logging"
)

// PerSpaceCronConfig holds per-space interval overrides stored on disk.
type PerSpaceCronConfig struct {
	SpaceKey          string `json:"space_key"`
	SpaceURL          string `json:"space_url"`
	FullCrawl         bool   `json:"full_crawl_enabled"`
	FullCrawlInterval string `json:"full_crawl_interval"`
	IncrCrawl         bool   `json:"incr_crawl_enabled"`
	IncrCrawlInterval string `json:"incr_crawl_interval"`
	Detection         string `json:"detection"`
}

type Manager struct {
	filePath string
	mu       sync.RWMutex
	log      logging.Sugar
	configs  []PerSpaceCronConfig
}

func NewManager(filePath string, log logging.Sugar) *Manager {
	m := &Manager{
		filePath: filePath,
		configs:  []PerSpaceCronConfig{},
		log:      log,
	}
	m.load()
	return m
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if m.log.Enabled() {
			m.log.Debugw("cron config file not found, starting empty", "path", m.filePath)
		}
		m.configs = []PerSpaceCronConfig{}
		return
	}
	var configs []PerSpaceCronConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		if m.log.Enabled() {
			m.log.Warnw("failed to parse cron config", "error", err)
		}
		m.configs = []PerSpaceCronConfig{}
		return
	}
	m.configs = configs
}

func (m *Manager) save() error {
	dir := m.filePath
	for i := len(m.filePath) - 1; i >= 0; i-- {
		if m.filePath[i] == '/' {
			dir = m.filePath[:i]
			break
		}
	}
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	data, err := json.MarshalIndent(m.configs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// GetOverride returns per-space cron override, or nil if not configured.
func (m *Manager) GetOverride(spaceKey string) *PerSpaceCronConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.configs {
		if c.SpaceKey == spaceKey {
			cp := c
			return &cp
		}
	}
	return nil
}

// GetSpaceURL returns the URL for a space key from overrides.
func (m *Manager) GetSpaceURL(spaceKey string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.configs {
		if c.SpaceKey == spaceKey {
			return c.SpaceURL
		}
	}
	return ""
}

// Upsert saves or updates a per-space cron config.
func (m *Manager) Upsert(cfg PerSpaceCronConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, c := range m.configs {
		if c.SpaceKey == cfg.SpaceKey {
			m.configs[i] = cfg
			return m.save()
		}
	}
	m.configs = append(m.configs, cfg)
	return m.save()
}

// Delete removes a per-space cron config.
func (m *Manager) Delete(spaceKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var kept []PerSpaceCronConfig
	for _, c := range m.configs {
		if c.SpaceKey != spaceKey {
			kept = append(kept, c)
		}
	}
	m.configs = kept
	return m.save()
}

// List returns all per-space configs.
func (m *Manager) List() []PerSpaceCronConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PerSpaceCronConfig, len(m.configs))
	copy(out, m.configs)
	return out
}
