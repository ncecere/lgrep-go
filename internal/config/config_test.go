package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)

	// Embeddings defaults
	assert.Equal(t, DefaultEmbeddingProvider, cfg.Embeddings.Provider)
	assert.Equal(t, DefaultOllamaURL, cfg.Embeddings.Ollama.URL)
	assert.Equal(t, DefaultOllamaEmbedModel, cfg.Embeddings.Ollama.Model)
	assert.Equal(t, DefaultOpenAIEmbedModel, cfg.Embeddings.OpenAI.Model)

	// LLM defaults
	assert.Equal(t, DefaultLLMProvider, cfg.LLM.Provider)
	assert.Equal(t, DefaultOllamaLLMModel, cfg.LLM.Ollama.Model)
	assert.Equal(t, DefaultOpenAILLMModel, cfg.LLM.OpenAI.Model)
	assert.Equal(t, DefaultAnthropicModel, cfg.LLM.Anthropic.Model)

	// Indexing defaults
	assert.Equal(t, DefaultMaxFileSize, cfg.Indexing.MaxFileSize)
	assert.Equal(t, DefaultMaxFileCount, cfg.Indexing.MaxFileCount)
	assert.Equal(t, DefaultChunkSize, cfg.Indexing.ChunkSize)
	assert.Equal(t, DefaultChunkOverlap, cfg.Indexing.ChunkOverlap)

	// Ignore patterns
	assert.NotEmpty(t, cfg.Ignore)
	assert.Contains(t, cfg.Ignore, "node_modules/")
	assert.Contains(t, cfg.Ignore, ".git/")
}

func TestDefaultIgnorePatterns(t *testing.T) {
	patterns := DefaultIgnorePatterns()

	assert.NotEmpty(t, patterns)

	// Check for common patterns
	expectedPatterns := []string{
		"*.lock",
		"node_modules/",
		".git/",
		"dist/",
		"*.exe",
		".DS_Store",
	}

	for _, expected := range expectedPatterns {
		assert.Contains(t, patterns, expected, "Expected pattern %s not found", expected)
	}
}

func TestDefaultPaths(t *testing.T) {
	configDir := DefaultConfigDir()
	dataDir := DefaultDataDir()
	dbPath := DefaultDatabasePath()

	assert.NotEmpty(t, configDir)
	assert.NotEmpty(t, dataDir)
	assert.NotEmpty(t, dbPath)

	// Should contain "lgrep"
	assert.Contains(t, configDir, "lgrep")
	assert.Contains(t, dataDir, "lgrep")
	assert.Contains(t, dbPath, "index.db")
}

func TestLoadWithConfigFile(t *testing.T) {
	// Reset viper and global config
	viper.Reset()
	cfg = nil

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
embeddings:
  provider: openai
  ollama:
    url: http://custom:11434
    model: custom-model
  openai:
    model: text-embedding-3-large
    base_url: https://custom-api.example.com
database:
  path: /custom/path/index.db
indexing:
  max_file_size: 2097152
  chunk_size: 1000
llm:
  provider: anthropic
  anthropic:
    model: claude-3-opus-20240229
ignore:
  - "custom-ignore/"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load the config
	err = Load(configPath)
	require.NoError(t, err)

	loadedCfg := Get()

	// Verify loaded values
	assert.Equal(t, "openai", loadedCfg.Embeddings.Provider)
	assert.Equal(t, "http://custom:11434", loadedCfg.Embeddings.Ollama.URL)
	assert.Equal(t, "custom-model", loadedCfg.Embeddings.Ollama.Model)
	assert.Equal(t, "text-embedding-3-large", loadedCfg.Embeddings.OpenAI.Model)
	assert.Equal(t, "https://custom-api.example.com", loadedCfg.Embeddings.OpenAI.BaseURL)
	assert.Equal(t, "/custom/path/index.db", loadedCfg.Database.Path)
	assert.Equal(t, 2097152, loadedCfg.Indexing.MaxFileSize)
	assert.Equal(t, 1000, loadedCfg.Indexing.ChunkSize)
	assert.Equal(t, "anthropic", loadedCfg.LLM.Provider)
	assert.Equal(t, "claude-3-opus-20240229", loadedCfg.LLM.Anthropic.Model)
	assert.Contains(t, loadedCfg.Ignore, "custom-ignore/")
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	// Reset viper and global config
	viper.Reset()
	cfg = nil

	// Set environment variables
	t.Setenv("LGREP_EMBEDDINGS_PROVIDER", "openai")
	t.Setenv("LGREP_LLM_PROVIDER", "anthropic")
	t.Setenv("OPENAI_API_KEY", "test-api-key")
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	// Load without a config file
	err := Load("")
	require.NoError(t, err)

	loadedCfg := Get()

	// Verify environment variables are loaded
	assert.Equal(t, "openai", loadedCfg.Embeddings.Provider)
	assert.Equal(t, "anthropic", loadedCfg.LLM.Provider)
	assert.Equal(t, "test-api-key", loadedCfg.Embeddings.OpenAI.APIKey)
	assert.Equal(t, "test-api-key", loadedCfg.LLM.OpenAI.APIKey)
	assert.Equal(t, "test-anthropic-key", loadedCfg.LLM.Anthropic.APIKey)
}

func TestLoadMissingConfigFile(t *testing.T) {
	// Reset viper and global config
	viper.Reset()
	cfg = nil

	// Load with non-existent config file - should not error, just use defaults
	err := Load("")
	require.NoError(t, err)

	loadedCfg := Get()

	// Should have default values
	assert.Equal(t, DefaultEmbeddingProvider, loadedCfg.Embeddings.Provider)
	assert.Equal(t, DefaultLLMProvider, loadedCfg.LLM.Provider)
}

func TestGet(t *testing.T) {
	// Reset global config
	cfg = nil

	// First call should return default config
	c1 := Get()
	assert.NotNil(t, c1)

	// Subsequent call should return same instance
	c2 := Get()
	assert.Same(t, c1, c2)
}

func TestGlobalConfigPath(t *testing.T) {
	path := GlobalConfigPath()
	assert.Contains(t, path, "lgrep")
	assert.Contains(t, path, "config.yaml")
}
