// Package config handles configuration loading and validation for lgrep.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

// Config represents the complete lgrep configuration.
type Config struct {
	Embeddings EmbeddingsConfig `mapstructure:"embeddings"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Indexing   IndexingConfig   `mapstructure:"indexing"`
	LLM        LLMConfig        `mapstructure:"llm"`
	Ignore     []string         `mapstructure:"ignore"`
}

// EmbeddingsConfig configures the embedding service.
type EmbeddingsConfig struct {
	Provider string            `mapstructure:"provider"`
	Ollama   OllamaEmbedConfig `mapstructure:"ollama"`
	OpenAI   OpenAIEmbedConfig `mapstructure:"openai"`
}

// OllamaEmbedConfig configures Ollama embeddings.
type OllamaEmbedConfig struct {
	URL   string `mapstructure:"url"`
	Model string `mapstructure:"model"`
}

// OpenAIEmbedConfig configures OpenAI embeddings.
type OpenAIEmbedConfig struct {
	Model      string `mapstructure:"model"`
	BaseURL    string `mapstructure:"base_url"`
	APIKey     string `mapstructure:"api_key"`
	Dimensions int    `mapstructure:"dimensions"`
}

// DatabaseConfig configures the SQLite database.
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// IndexingConfig configures the indexing process.
type IndexingConfig struct {
	MaxFileSize  int `mapstructure:"max_file_size"`
	MaxFileCount int `mapstructure:"max_file_count"`
	ChunkSize    int `mapstructure:"chunk_size"`
	ChunkOverlap int `mapstructure:"chunk_overlap"`
}

// LLMConfig configures the LLM service for Q&A.
type LLMConfig struct {
	Provider  string          `mapstructure:"provider"`
	Ollama    OllamaLLMConfig `mapstructure:"ollama"`
	OpenAI    OpenAILLMConfig `mapstructure:"openai"`
	Anthropic AnthropicConfig `mapstructure:"anthropic"`
}

// OllamaLLMConfig configures Ollama LLM.
type OllamaLLMConfig struct {
	URL   string `mapstructure:"url"`
	Model string `mapstructure:"model"`
}

// OpenAILLMConfig configures OpenAI LLM.
type OpenAILLMConfig struct {
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
}

// AnthropicConfig configures Anthropic LLM.
type AnthropicConfig struct {
	Model  string `mapstructure:"model"`
	APIKey string `mapstructure:"api_key"`
}

// Global configuration instance
var cfg *Config

// Get returns the current configuration.
func Get() *Config {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return cfg
}

// DefaultConfig returns a configuration with default values.
func DefaultConfig() *Config {
	return &Config{
		Embeddings: EmbeddingsConfig{
			Provider: DefaultEmbeddingProvider,
			Ollama: OllamaEmbedConfig{
				URL:   DefaultOllamaURL,
				Model: DefaultOllamaEmbedModel,
			},
			OpenAI: OpenAIEmbedConfig{
				Model: DefaultOpenAIEmbedModel,
			},
		},
		Database: DatabaseConfig{
			Path: DefaultDatabasePath(),
		},
		Indexing: IndexingConfig{
			MaxFileSize:  DefaultMaxFileSize,
			MaxFileCount: DefaultMaxFileCount,
			ChunkSize:    DefaultChunkSize,
			ChunkOverlap: DefaultChunkOverlap,
		},
		LLM: LLMConfig{
			Provider: DefaultLLMProvider,
			Ollama: OllamaLLMConfig{
				URL:   DefaultOllamaURL,
				Model: DefaultOllamaLLMModel,
			},
			OpenAI: OpenAILLMConfig{
				Model: DefaultOpenAILLMModel,
			},
			Anthropic: AnthropicConfig{
				Model: DefaultAnthropicModel,
			},
		},
		Ignore: DefaultIgnorePatterns(),
	}
}

// Load reads configuration from file and environment variables.
func Load(configFile string) error {
	// Set defaults
	setDefaults()

	// Set config file if specified
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Search for config in standard locations
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(DefaultConfigDir())
		viper.AddConfigPath(".")

		// Also check for .lgreprc.yaml in current directory and parents
		if rcPath := findRCFile(); rcPath != "" {
			viper.SetConfigFile(rcPath)
		}
	}

	// Environment variables
	viper.SetEnvPrefix("LGREP")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
		log.Debug("No config file found, using defaults")
	} else {
		log.Debug("Loaded config from", "file", viper.ConfigFileUsed())
	}

	// Unmarshal into config struct
	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("error parsing config: %w", err)
	}

	// Load API keys from environment if not in config
	loadAPIKeysFromEnv()

	return nil
}

// setDefaults sets default values in viper.
func setDefaults() {
	// Embeddings
	viper.SetDefault("embeddings.provider", DefaultEmbeddingProvider)
	viper.SetDefault("embeddings.ollama.url", DefaultOllamaURL)
	viper.SetDefault("embeddings.ollama.model", DefaultOllamaEmbedModel)
	viper.SetDefault("embeddings.openai.model", DefaultOpenAIEmbedModel)

	// Database
	viper.SetDefault("database.path", DefaultDatabasePath())

	// Indexing
	viper.SetDefault("indexing.max_file_size", DefaultMaxFileSize)
	viper.SetDefault("indexing.max_file_count", DefaultMaxFileCount)
	viper.SetDefault("indexing.chunk_size", DefaultChunkSize)
	viper.SetDefault("indexing.chunk_overlap", DefaultChunkOverlap)

	// LLM
	viper.SetDefault("llm.provider", DefaultLLMProvider)
	viper.SetDefault("llm.ollama.url", DefaultOllamaURL)
	viper.SetDefault("llm.ollama.model", DefaultOllamaLLMModel)
	viper.SetDefault("llm.openai.model", DefaultOpenAILLMModel)
	viper.SetDefault("llm.anthropic.model", DefaultAnthropicModel)

	// Ignore patterns
	viper.SetDefault("ignore", DefaultIgnorePatterns())
}

// findRCFile searches for .lgreprc.yaml starting from current directory.
func findRCFile() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := cwd
	for {
		rcPath := filepath.Join(dir, ".lgreprc.yaml")
		if _, err := os.Stat(rcPath); err == nil {
			return rcPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// loadAPIKeysFromEnv loads API keys from environment variables if not already set.
func loadAPIKeysFromEnv() {
	// OpenAI API key
	if cfg.Embeddings.OpenAI.APIKey == "" {
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			cfg.Embeddings.OpenAI.APIKey = key
		}
	}
	if cfg.LLM.OpenAI.APIKey == "" {
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			cfg.LLM.OpenAI.APIKey = key
		}
	}

	// Anthropic API key
	if cfg.LLM.Anthropic.APIKey == "" {
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			cfg.LLM.Anthropic.APIKey = key
		}
	}
}

// ConfigFilePath returns the path of the loaded config file, or empty string if none.
func ConfigFilePath() string {
	return viper.ConfigFileUsed()
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}
