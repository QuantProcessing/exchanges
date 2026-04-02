package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
	"gopkg.in/yaml.v3"
)

// Config is the top-level YAML/JSON shape for adapter bootstrap files.
type Config struct {
	Exchanges []ExchangeConfig `json:"exchanges" yaml:"exchanges"`
}

// ExchangeConfig describes one adapter instance to construct from config.
type ExchangeConfig struct {
	Name       string               `json:"name" yaml:"name"`
	Alias      string               `json:"alias,omitempty" yaml:"alias,omitempty"`
	MarketType exchanges.MarketType `json:"market_type" yaml:"market_type"`
	Options    map[string]string    `json:"options,omitempty" yaml:"options,omitempty"`
}

// Load reads a YAML or JSON config file from disk and expands environment variables.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return parse(path, data)
}

// LoadManager loads a config file and constructs a populated adapter manager.
func LoadManager(ctx context.Context, path string) (*exchanges.Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	return BuildManager(ctx, cfg)
}

// BuildManager constructs a manager from the provided parsed configuration.
func BuildManager(ctx context.Context, cfg Config) (*exchanges.Manager, error) {
	manager := exchanges.NewManager()
	nameCounts := make(map[string]int, len(cfg.Exchanges))

	for _, item := range cfg.Exchanges {
		name := normalizeName(item.Name)
		if name != "" {
			nameCounts[name]++
		}
	}

	seenAliases := make(map[string]struct{}, len(cfg.Exchanges))
	for _, item := range cfg.Exchanges {
		name := normalizeName(item.Name)
		if name == "" {
			return nil, fmt.Errorf("exchange name is required")
		}

		marketType := normalizeMarketType(item.MarketType)
		if marketType == "" {
			return nil, fmt.Errorf("%s: market_type is required", name)
		}

		alias := strings.TrimSpace(item.Alias)
		if alias == "" {
			alias = defaultAlias(name, marketType, nameCounts[name])
		}
		if _, exists := seenAliases[alias]; exists {
			return nil, fmt.Errorf("duplicate adapter alias %q", alias)
		}
		seenAliases[alias] = struct{}{}

		ctor, err := exchanges.LookupConstructor(name)
		if err != nil {
			return nil, err
		}

		opts := item.Options
		if opts == nil {
			opts = map[string]string{}
		}

		adp, err := ctor(ctx, marketType, opts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", alias, err)
		}

		manager.Register(alias, adp)
	}

	return manager, nil
}

func parse(path string, data []byte) (Config, error) {
	expanded := os.ExpandEnv(string(data))
	var cfg Config

	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
			return Config{}, err
		}
	case ".json":
		if err := json.Unmarshal([]byte(expanded), &cfg); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("unsupported config format %q", filepath.Ext(path))
	}

	return cfg, nil
}

func normalizeName(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}

func normalizeMarketType(mt exchanges.MarketType) exchanges.MarketType {
	return exchanges.MarketType(strings.ToLower(strings.TrimSpace(string(mt))))
}

func defaultAlias(name string, marketType exchanges.MarketType, sameNameCount int) string {
	if sameNameCount <= 1 {
		return name
	}
	return fmt.Sprintf("%s/%s", name, marketType)
}
