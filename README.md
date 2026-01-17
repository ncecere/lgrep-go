# lgrep

A privacy-focused local semantic code search CLI. Search your codebase using natural language and get AI-powered answers.

## Features

- **Local-first**: Run embeddings locally with Ollama or use cloud providers (OpenAI)
- **Semantic search**: Find code by meaning, not just keywords
- **Q&A mode**: Get AI-generated answers about your codebase with source citations
- **Fast**: SQLite + sqlite-vec for efficient vector storage and search
- **Privacy-focused**: Your code never leaves your machine (when using Ollama)
- **Code-aware chunking**: Intelligently splits code at function/class boundaries
- **Multi-provider support**: Ollama, OpenAI, and Anthropic for LLM

## Installation

### Download Binary (Recommended)

Download the appropriate binary from GitHub Releases:

| Platform | Architecture | Download |
| --- | --- | --- |
| Linux | amd64 | `lgrep-linux-amd64.tar.gz` |
| Linux | arm64 | `lgrep-linux-arm64.tar.gz` |
| macOS | Apple Silicon | `lgrep-darwin-arm64.tar.gz` |

Example (macOS Apple Silicon):

```bash
curl -L https://github.com/nickcecere/lgrep/releases/latest/download/lgrep-darwin-arm64.tar.gz | tar xz
sudo mv lgrep-darwin-arm64 /usr/local/bin/lgrep
chmod +x /usr/local/bin/lgrep
```

### From Source

```bash
# Clone and build
git clone https://github.com/nickcecere/lgrep.git
cd lgrep-go
make build

# Install to /usr/local/bin
make install
```

### Using Go

```bash
go install github.com/nickcecere/lgrep/cmd/lgrep@latest
```

### Using Homebrew (coming soon)

```bash
brew install nickcecere/tap/lgrep
```

## Quick Start

```bash
# 1. Start Ollama and pull the embedding model
ollama pull nomic-embed-text

# 2. Index the current directory
lgrep index

# 3. Search for code
lgrep search "how does authentication work"

# 4. Search with Q&A mode (requires LLM model)
ollama pull llama3.2
lgrep search "how are errors handled" -a

# 5. Show code snippets in results
lgrep search "database queries" -c
```

## Commands

### `lgrep index [path]`

Index files in a directory for semantic search.

```bash
# Index current directory
lgrep index

# Index specific directory
lgrep index ./src

# Index only specific file types
lgrep index --ext .go --ext .ts

# Preview what would be indexed (dry run)
lgrep index --dry-run

# Force re-index all files
lgrep index --force
```

**Flags:**
- `-f, --force` - Force re-index all files
- `-d, --dry-run` - Preview without indexing
- `-e, --ext` - File extensions to include (can be repeated)
- `-i, --ignore` - Additional patterns to ignore
- `--store` - Custom store name

### `lgrep search <query>`

Search indexed files using semantic similarity.

```bash
# Basic search
lgrep search "how does the cache work"

# Show code snippets
lgrep search "error handling" -c

# Limit results
lgrep search "api endpoints" -m 5

# Filter by similarity score
lgrep search "authentication" --min-score 0.5

# JSON output
lgrep search "database" --json

# Q&A mode - get an AI-generated answer
lgrep search "how does authentication work" -a
```

**Flags:**
- `-c, --content` - Show code snippets in results
- `-a, --answer` - Generate an answer using LLM (Q&A mode)
- `-m, --limit` - Maximum number of results (default: 10)
- `--min-score` - Minimum similarity score (0-1)
- `--context` - Lines of context to show
- `--json` - Output results as JSON
- `--store` - Search specific store

### `lgrep status`

Show index status and statistics.

```bash
# Show status for current directory
lgrep status

# Show all stores
lgrep status --all

# Show specific store
lgrep status --store myproject
```

### `lgrep list`

List all indexed stores.

```bash
lgrep list
```

### `lgrep delete <store>`

Delete an indexed store and all its data.

```bash
lgrep delete myproject
```

### `lgrep config`

Show current configuration.

```bash
# Show all config
lgrep config

# Show config file path
lgrep config --path
```

## Configuration

Configuration is loaded from (in order of precedence):
1. Command-line flags
2. Environment variables (`LGREP_*`)
3. Local config: `.lgreprc.yaml` (searched from cwd upward)
4. Global config: `~/.config/lgrep/config.yaml`

### Example Configuration

```yaml
# ~/.config/lgrep/config.yaml

# Embedding provider for indexing and search
embeddings:
  provider: ollama  # or "openai"
  ollama:
    url: http://localhost:11434
    model: nomic-embed-text  # or mxbai-embed-large
  openai:
    model: text-embedding-3-small
    # api_key: set via OPENAI_API_KEY env var

# LLM provider for Q&A mode
llm:
  provider: ollama  # or "openai" or "anthropic"
  ollama:
    url: http://localhost:11434
    model: llama3.2
  openai:
    model: gpt-4o
  anthropic:
    model: claude-3-5-sonnet-20241022

# Database location
database:
  path: ~/.local/share/lgrep/index.db

# Indexing settings
indexing:
  max_file_size: 1048576  # 1MB
  max_file_count: 10000
  chunk_size: 1500
  chunk_overlap: 200

# Additional ignore patterns (gitignore syntax)
ignore:
  - "*.log"
  - "tmp/"
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `LGREP_EMBEDDINGS_PROVIDER` | Embedding provider (ollama/openai) |
| `LGREP_LLM_PROVIDER` | LLM provider (ollama/openai/anthropic) |
| `OPENAI_API_KEY` | OpenAI API key |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `LGREP_DATABASE_PATH` | Database file location |

## Supported Models

### Embedding Models

| Provider | Model | Dimensions | Notes |
|----------|-------|------------|-------|
| Ollama | `nomic-embed-text` | 768 | Recommended, fast |
| Ollama | `mxbai-embed-large` | 1024 | Higher quality |
| OpenAI | `text-embedding-3-small` | 1536 | Good balance |
| OpenAI | `text-embedding-3-large` | 3072 | Highest quality |

### LLM Models for Q&A

| Provider | Model | Notes |
|----------|-------|-------|
| Ollama | `llama3.2` | Fast, good for code |
| Ollama | `codellama` | Optimized for code |
| OpenAI | `gpt-4o` | Best quality |
| OpenAI | `gpt-4o-mini` | Fast and cheap |
| Anthropic | `claude-3-5-sonnet` | Excellent for code |

## Development

```bash
# Run tests
make test

# Run with verbose output
make test-verbose

# Run with debug logging
./bin/lgrep --debug search "test query"

# Build for current platform
make build

# Build for all platforms
goreleaser build --snapshot --clean

# Create release
goreleaser release --snapshot --clean
```

### Project Structure

```
lgrep-go/
├── cmd/lgrep/          # CLI entry point
├── internal/
│   ├── cli/            # Command implementations
│   ├── config/         # Configuration loading
│   ├── embeddings/     # Embedding services (Ollama, OpenAI)
│   ├── fs/             # File walking, chunking, language detection
│   ├── indexer/        # Indexing orchestration
│   ├── llm/            # LLM services (Ollama, OpenAI, Anthropic)
│   ├── search/         # Semantic search
│   ├── store/          # SQLite + sqlite-vec storage
│   └── ui/             # Terminal styling
├── Makefile
├── .goreleaser.yaml
└── go.mod
```

## Requirements

### For Local Use (Recommended)

Install [Ollama](https://ollama.ai/) and pull the required models:

```bash
# Install Ollama (macOS)
brew install ollama

# Or download from https://ollama.ai/

# Pull embedding model
ollama pull nomic-embed-text

# Pull LLM model for Q&A (optional)
ollama pull llama3.2
```

### For Cloud Providers

Set your API keys:

```bash
# OpenAI
export OPENAI_API_KEY=sk-...

# Anthropic
export ANTHROPIC_API_KEY=sk-ant-...
```

## How It Works

1. **Indexing**: Files are walked, chunked (respecting code boundaries), and embedded using the configured model. Embeddings are stored in SQLite using sqlite-vec.

2. **Search**: Your query is embedded using the same model, then sqlite-vec performs a fast cosine similarity search to find the most relevant chunks.

3. **Q&A Mode**: Search results are used as context for an LLM, which generates a natural language answer with source citations.

## Comparison with Similar Tools

| Feature | lgrep | grep | ripgrep | GitHub Copilot |
|---------|-------|------|---------|----------------|
| Semantic search | ✅ | ❌ | ❌ | ✅ |
| Local/private | ✅ | ✅ | ✅ | ❌ |
| Q&A mode | ✅ | ❌ | ❌ | ✅ |
| Fast keyword search | ❌ | ✅ | ✅ | ❌ |
| No setup required | ❌ | ✅ | ✅ | ❌ |

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
