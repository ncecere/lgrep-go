// Package fs provides file system operations for indexing.
package fs

import (
	"io"
	"time"
)

// FileInfo represents metadata about a file.
type FileInfo struct {
	Path     string    // Absolute path to the file
	RelPath  string    // Path relative to the root
	Size     int64     // File size in bytes
	ModTime  time.Time // Last modification time
	Hash     string    // xxhash of file contents
	Language string    // Detected programming language (if applicable)
}

// Chunk represents a piece of a file for embedding.
type Chunk struct {
	Content    string // The text content of the chunk
	StartLine  int    // Starting line number (1-indexed)
	EndLine    int    // Ending line number (1-indexed)
	StartChar  int    // Starting character offset
	EndChar    int    // Ending character offset
	ChunkIndex int    // Index of this chunk within the file
}

// WalkOptions configures the file walker.
type WalkOptions struct {
	// Root is the directory to start walking from.
	Root string

	// MaxFileSize is the maximum file size to process (in bytes).
	MaxFileSize int64

	// MaxFileCount is the maximum number of files to process.
	MaxFileCount int

	// IgnorePatterns are additional patterns to ignore (gitignore syntax).
	IgnorePatterns []string

	// IncludeHidden includes hidden files and directories.
	IncludeHidden bool

	// UseGitignore respects .gitignore files.
	UseGitignore bool

	// Extensions limits to specific file extensions (e.g., ".go", ".ts").
	// Empty means all text files.
	Extensions []string
}

// ChunkOptions configures the chunker.
type ChunkOptions struct {
	// ChunkSize is the target size for each chunk in characters.
	ChunkSize int

	// ChunkOverlap is the number of overlapping characters between chunks.
	ChunkOverlap int

	// MinChunkSize is the minimum chunk size. Smaller chunks are merged.
	MinChunkSize int
}

// DefaultWalkOptions returns sensible defaults for walking.
func DefaultWalkOptions() WalkOptions {
	return WalkOptions{
		MaxFileSize:  1024 * 1024, // 1MB
		MaxFileCount: 10000,
		UseGitignore: true,
	}
}

// DefaultChunkOptions returns sensible defaults for chunking.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{
		ChunkSize:    1500,
		ChunkOverlap: 200,
		MinChunkSize: 100,
	}
}

// Walker walks a directory tree and yields files.
type Walker interface {
	// Walk walks the directory tree and calls fn for each file.
	// The walk stops if fn returns an error.
	Walk(fn func(FileInfo) error) error

	// Stats returns statistics about the walk.
	Stats() WalkStats
}

// WalkStats contains statistics from a directory walk.
type WalkStats struct {
	FilesFound   int   // Total files found
	FilesSkipped int   // Files skipped due to size/pattern/etc
	DirsSkipped  int   // Directories skipped
	TotalBytes   int64 // Total bytes of files found
	SkippedBytes int64 // Total bytes of skipped files
}

// Chunker splits file contents into chunks.
type Chunker interface {
	// Chunk splits the content into chunks.
	Chunk(content string, filename string) []Chunk

	// ChunkReader splits content from a reader into chunks.
	ChunkReader(r io.Reader, filename string) ([]Chunk, error)
}
