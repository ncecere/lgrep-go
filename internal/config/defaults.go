package config

import (
	"os"
	"path/filepath"
)

// Default configuration values
const (
	// Embedding defaults
	DefaultEmbeddingProvider = "ollama"
	DefaultOllamaURL         = "http://localhost:11434"
	DefaultOllamaEmbedModel  = "nomic-embed-text"
	DefaultOpenAIEmbedModel  = "text-embedding-3-small"

	// LLM defaults
	DefaultLLMProvider    = "ollama"
	DefaultOllamaLLMModel = "llama3"
	DefaultOpenAILLMModel = "gpt-4o-mini"
	DefaultAnthropicModel = "claude-3-haiku-20240307"

	// Indexing defaults
	DefaultMaxFileSize  = 1 << 20 // 1MB
	DefaultMaxFileCount = 10000
	DefaultChunkSize    = 500
	DefaultChunkOverlap = 50

	// Database
	DefaultDBFileName = "index.db"
)

// DefaultIgnorePatterns returns the default list of file patterns to ignore.
func DefaultIgnorePatterns() []string {
	return []string{
		// Lock files
		"*.lock",
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"Cargo.lock",
		"go.sum",
		"poetry.lock",
		"Gemfile.lock",

		// Build outputs
		"dist/",
		"build/",
		"out/",
		"target/",
		"__pycache__/",
		"*.pyc",
		".next/",
		".nuxt/",

		// Dependencies
		"node_modules/",
		"vendor/",
		".venv/",
		"venv/",

		// IDE/Editor
		".idea/",
		".vscode/",
		"*.swp",
		"*.swo",
		"*~",

		// Version control
		".git/",
		".svn/",
		".hg/",

		// Binary/compiled
		"*.exe",
		"*.dll",
		"*.so",
		"*.dylib",
		"*.o",
		"*.a",
		"*.class",

		// Media/Binary
		"*.jpg",
		"*.jpeg",
		"*.png",
		"*.gif",
		"*.ico",
		"*.svg",
		"*.webp",
		"*.mp3",
		"*.mp4",
		"*.wav",
		"*.avi",
		"*.mov",
		"*.pdf",
		"*.doc",
		"*.docx",
		"*.xls",
		"*.xlsx",

		// Archives
		"*.zip",
		"*.tar",
		"*.tar.gz",
		"*.tgz",
		"*.rar",
		"*.7z",

		// Minified
		"*.min.js",
		"*.min.css",
		"*.map",

		// Misc
		".DS_Store",
		"Thumbs.db",
		".env",
		".env.*",
		"*.log",
	}
}

// DefaultConfigDir returns the default configuration directory path.
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".config/lgrep"
	}
	return filepath.Join(home, ".config", "lgrep")
}

// DefaultDataDir returns the default data directory path.
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".local/share/lgrep"
	}
	return filepath.Join(home, ".local", "share", "lgrep")
}

// DefaultDatabasePath returns the default database file path.
func DefaultDatabasePath() string {
	return filepath.Join(DefaultDataDir(), DefaultDBFileName)
}
