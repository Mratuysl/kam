package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AIProvider string

const (
	ProviderClaude     AIProvider = "claude"
	ProviderOpenAI     AIProvider = "openai"
	ProviderOllama     AIProvider = "ollama"
	ProviderReplicate  AIProvider = "replicate"
)

type Config struct {
	AI         AIConfig  `yaml:"ai"`
	Kubernetes K8sConfig `yaml:"kubernetes"`
	UI         UIConfig  `yaml:"ui"`
}

type AIConfig struct {
	Provider   AIProvider `yaml:"provider"`    // claude, openai, ollama
	APIKey     string     `yaml:"api_key"`     // boş bırakılabilir, env var kullanılır
	Model      string     `yaml:"model"`       // örn: claude-sonnet-4-20250514
	OllamaURL  string     `yaml:"ollama_url"`  // local ollama için
	MaxTokens  int        `yaml:"max_tokens"`
}

type K8sConfig struct {
	KubeconfigPath string `yaml:"kubeconfig"`  // default: ~/.kube/config
	DefaultNS      string `yaml:"default_namespace"`
}

type UIConfig struct {
	Theme      string `yaml:"theme"`        // dark, light
	DateFormat string `yaml:"date_format"`  // relative, absolute
	AgentName  string `yaml:"agent_name"`   // kullanıcının ajanına verdiği isim (varsayılan: kam)
}

// DefaultConfig sensible defaults döndürür
func DefaultConfig() *Config {
	return &Config{
		AI: AIConfig{
			Provider:  ProviderClaude,
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 2048,
			OllamaURL: "http://localhost:11434",
		},
		Kubernetes: K8sConfig{
			DefaultNS: "default",
		},
		UI: UIConfig{
			Theme:      "dark",
			DateFormat: "relative",
			AgentName:  "kam",
		},
	}
}

// Load config dosyasını okur, yoksa default döndürür
func Load() (*Config, error) {
	cfg := DefaultConfig()

	path, err := configPath()
	if err != nil {
		return cfg, nil // config yoksa default kullan
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config parse hatası: %w", err)
	}

	// API key env var'dan da alınabilsin
	if cfg.AI.APIKey == "" {
		switch cfg.AI.Provider {
		case ProviderClaude:
			cfg.AI.APIKey = os.Getenv("ANTHROPIC_API_KEY")
		case ProviderOpenAI:
			cfg.AI.APIKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	return cfg, nil
}

// Save config dosyasına yazar
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kam", "config.yaml"), nil
}
