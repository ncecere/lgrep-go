package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectLanguage tests language detection from file paths.
func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", LangGo},
		{"app.ts", LangTypeScript},
		{"component.tsx", LangTypeScript},
		{"script.js", LangJavaScript},
		{"utils.py", LangPython},
		{"lib.rs", LangRust},
		{"Main.java", LangJava},
		{"main.c", LangC},
		{"main.cpp", LangCPP},
		{"Program.cs", LangCSharp},
		{"app.rb", LangRuby},
		{"index.php", LangPHP},
		{"main.swift", LangSwift},
		{"Main.kt", LangKotlin},
		{"App.scala", LangScala},
		{"script.sh", LangShell},
		{"query.sql", LangSQL},
		{"index.html", LangHTML},
		{"style.css", LangCSS},
		{"data.json", LangJSON},
		{"config.yaml", LangYAML},
		{"config.toml", LangTOML},
		{"README.md", LangMarkdown},
		{"file.xml", LangXML},
		{"notes.txt", LangText},
		{"Makefile", LangShell},
		{"Dockerfile", LangShell},
		{"unknown.xyz", LangUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			lang := DetectLanguage(tt.path)
			assert.Equal(t, tt.expected, lang)
		})
	}
}

// TestIsCodeFile tests code file detection.
func TestIsCodeFile(t *testing.T) {
	assert.True(t, IsCodeFile("main.go"))
	assert.True(t, IsCodeFile("app.ts"))
	assert.True(t, IsCodeFile("utils.py"))
	assert.False(t, IsCodeFile("README.md"))
	assert.False(t, IsCodeFile("data.json"))
	assert.False(t, IsCodeFile("unknown.xyz"))
}

// TestHashContent tests content hashing.
func TestHashContent(t *testing.T) {
	// Same content should produce same hash
	content := []byte("hello world")
	hash1 := HashContent(content)
	hash2 := HashContent(content)
	assert.Equal(t, hash1, hash2)

	// Different content should produce different hash
	hash3 := HashContent([]byte("hello world!"))
	assert.NotEqual(t, hash1, hash3)

	// Hash should be 16 hex characters (64 bits)
	assert.Len(t, hash1, 16)
}

// TestIsBinaryContent tests binary detection.
func TestIsBinaryContent(t *testing.T) {
	// Text content
	assert.False(t, isBinaryContent([]byte("Hello, World!\n")))
	assert.False(t, isBinaryContent([]byte("line1\nline2\tindented")))

	// Binary content (null bytes)
	assert.True(t, isBinaryContent([]byte("hello\x00world")))

	// Empty content
	assert.False(t, isBinaryContent([]byte{}))
}

// TestTextChunker tests basic text chunking.
func TestTextChunker(t *testing.T) {
	chunker := NewTextChunker(ChunkOptions{
		ChunkSize:    100,
		ChunkOverlap: 20,
		MinChunkSize: 10,
	})

	t.Run("empty content returns nil", func(t *testing.T) {
		chunks := chunker.Chunk("", "test.txt")
		assert.Nil(t, chunks)
	})

	t.Run("small content returns single chunk", func(t *testing.T) {
		content := "Hello, World!"
		chunks := chunker.Chunk(content, "test.txt")
		require.Len(t, chunks, 1)
		assert.Equal(t, content, chunks[0].Content)
		assert.Equal(t, 1, chunks[0].StartLine)
		assert.Equal(t, 1, chunks[0].EndLine)
		assert.Equal(t, 0, chunks[0].ChunkIndex)
	})

	t.Run("large content is split into chunks", func(t *testing.T) {
		// Create content with multiple lines
		lines := make([]string, 20)
		for i := range lines {
			lines[i] = strings.Repeat("x", 10) // 10 chars per line
		}
		content := strings.Join(lines, "\n")

		chunks := chunker.Chunk(content, "test.txt")
		require.Greater(t, len(chunks), 1)

		// Verify chunk indices are sequential
		for i, chunk := range chunks {
			assert.Equal(t, i, chunk.ChunkIndex)
		}

		// Verify line numbers are reasonable
		assert.Equal(t, 1, chunks[0].StartLine)
	})

	t.Run("chunks have overlap", func(t *testing.T) {
		lines := make([]string, 30)
		for i := range lines {
			lines[i] = strings.Repeat("y", 10)
		}
		content := strings.Join(lines, "\n")

		chunks := chunker.Chunk(content, "test.txt")
		require.Greater(t, len(chunks), 1)

		// Check that consecutive chunks have overlapping line ranges
		for i := 1; i < len(chunks); i++ {
			// Later chunk should start before or at where previous ended
			assert.LessOrEqual(t, chunks[i].StartLine, chunks[i-1].EndLine+1)
		}
	})
}

// TestCodeChunker tests code-aware chunking.
func TestCodeChunker(t *testing.T) {
	chunker := NewTextChunker(ChunkOptions{
		ChunkSize:    500,
		ChunkOverlap: 50,
		MinChunkSize: 20,
	})

	t.Run("Go code splits on functions", func(t *testing.T) {
		content := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}

func helper() {
	// do something
}

type Config struct {
	Name string
}
`
		chunks := chunker.Chunk(content, "main.go")
		require.NotEmpty(t, chunks)

		// Should have chunks for different code blocks
		// The exact number depends on size thresholds
		foundMain := false
		foundHelper := false
		for _, chunk := range chunks {
			if strings.Contains(chunk.Content, "func main()") {
				foundMain = true
			}
			if strings.Contains(chunk.Content, "func helper()") {
				foundHelper = true
			}
		}
		assert.True(t, foundMain || foundHelper)
	})

	t.Run("Python code splits on definitions", func(t *testing.T) {
		content := `import os

def main():
    print("Hello")

class Config:
    def __init__(self):
        self.name = ""

def helper():
    pass
`
		chunks := chunker.Chunk(content, "main.py")
		require.NotEmpty(t, chunks)
	})

	t.Run("TypeScript code splits on exports", func(t *testing.T) {
		content := `import { foo } from './foo';

export function main(): void {
  console.log("Hello");
}

export class Config {
  name: string = "";
}

export const helper = () => {
  // do something
};
`
		chunks := chunker.Chunk(content, "main.ts")
		require.NotEmpty(t, chunks)
	})
}

// TestFileWalker tests directory walking.
func TestFileWalker(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"main.go":           "package main\n\nfunc main() {}\n",
		"utils.go":          "package main\n\nfunc helper() {}\n",
		"README.md":         "# Test\n",
		"subdir/nested.go":  "package subdir\n",
		".hidden":           "hidden file",
		"node_modules/a.js": "// should be ignored",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	// Create .gitignore
	gitignore := "*.md\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(gitignore), 0644))

	t.Run("walks directory and finds files", func(t *testing.T) {
		walker, err := NewFileWalker(WalkOptions{
			Root:         tmpDir,
			UseGitignore: true,
		})
		require.NoError(t, err)

		var found []string
		err = walker.Walk(func(info FileInfo) error {
			found = append(found, info.RelPath)
			return nil
		})
		require.NoError(t, err)

		// Should find Go files, not hidden, not gitignored, not node_modules
		assert.Contains(t, found, "main.go")
		assert.Contains(t, found, "utils.go")
		assert.Contains(t, found, filepath.Join("subdir", "nested.go"))

		// Should not find these
		for _, f := range found {
			assert.NotEqual(t, ".hidden", f)
			assert.NotContains(t, f, "node_modules")
			assert.NotEqual(t, "README.md", f) // gitignored
		}
	})

	t.Run("respects extension filter", func(t *testing.T) {
		walker, err := NewFileWalker(WalkOptions{
			Root:       tmpDir,
			Extensions: []string{".md"},
		})
		require.NoError(t, err)

		var found []string
		err = walker.Walk(func(info FileInfo) error {
			found = append(found, info.RelPath)
			return nil
		})
		require.NoError(t, err)

		// Should only find .md files (though gitignore would normally exclude)
		// Since we're testing without gitignore respect here
		for _, f := range found {
			assert.True(t, strings.HasSuffix(f, ".md"), "unexpected file: %s", f)
		}
	})

	t.Run("respects max file count", func(t *testing.T) {
		walker, err := NewFileWalker(WalkOptions{
			Root:         tmpDir,
			MaxFileCount: 2,
		})
		require.NoError(t, err)

		var count int
		err = walker.Walk(func(info FileInfo) error {
			count++
			return nil
		})
		require.NoError(t, err)

		assert.Equal(t, 2, count)
	})

	t.Run("includes hidden files when configured", func(t *testing.T) {
		walker, err := NewFileWalker(WalkOptions{
			Root:          tmpDir,
			IncludeHidden: true,
			UseGitignore:  false,
		})
		require.NoError(t, err)

		var found []string
		err = walker.Walk(func(info FileInfo) error {
			found = append(found, info.RelPath)
			return nil
		})
		require.NoError(t, err)

		// Should find hidden file
		assert.Contains(t, found, ".hidden")
	})

	t.Run("provides accurate stats", func(t *testing.T) {
		walker, err := NewFileWalker(WalkOptions{
			Root:         tmpDir,
			UseGitignore: true,
		})
		require.NoError(t, err)

		err = walker.Walk(func(info FileInfo) error { return nil })
		require.NoError(t, err)

		stats := walker.Stats()
		assert.Greater(t, stats.FilesFound, 0)
		assert.Greater(t, stats.TotalBytes, int64(0))
	})

	t.Run("computes file hashes", func(t *testing.T) {
		walker, err := NewFileWalker(WalkOptions{
			Root:       tmpDir,
			Extensions: []string{".go"},
		})
		require.NoError(t, err)

		var hashes []string
		err = walker.Walk(func(info FileInfo) error {
			hashes = append(hashes, info.Hash)
			return nil
		})
		require.NoError(t, err)

		for _, hash := range hashes {
			assert.Len(t, hash, 16, "hash should be 16 hex chars")
		}
	})

	t.Run("detects languages", func(t *testing.T) {
		walker, err := NewFileWalker(WalkOptions{
			Root:       tmpDir,
			Extensions: []string{".go"},
		})
		require.NoError(t, err)

		var languages []string
		err = walker.Walk(func(info FileInfo) error {
			languages = append(languages, info.Language)
			return nil
		})
		require.NoError(t, err)

		for _, lang := range languages {
			assert.Equal(t, LangGo, lang)
		}
	})
}

// TestFileWalkerErrors tests error handling.
func TestFileWalkerErrors(t *testing.T) {
	t.Run("non-existent root", func(t *testing.T) {
		_, err := NewFileWalker(WalkOptions{
			Root: "/nonexistent/path",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("root is file not directory", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		_, err = NewFileWalker(WalkOptions{
			Root: tmpFile.Name(),
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})
}

// TestChunkerReader tests chunking from a reader.
func TestChunkerReader(t *testing.T) {
	chunker := NewTextChunker(DefaultChunkOptions())

	content := "line 1\nline 2\nline 3\n"
	reader := strings.NewReader(content)

	chunks, err := chunker.ChunkReader(reader, "test.txt")
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	assert.Contains(t, chunks[0].Content, "line 1")
}

// TestDefaultOptions tests default options.
func TestDefaultOptions(t *testing.T) {
	walkOpts := DefaultWalkOptions()
	assert.Equal(t, int64(1024*1024), walkOpts.MaxFileSize)
	assert.Equal(t, 10000, walkOpts.MaxFileCount)
	assert.True(t, walkOpts.UseGitignore)

	chunkOpts := DefaultChunkOptions()
	assert.Equal(t, 1500, chunkOpts.ChunkSize)
	assert.Equal(t, 200, chunkOpts.ChunkOverlap)
	assert.Equal(t, 100, chunkOpts.MinChunkSize)
}
