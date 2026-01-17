package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/charmbracelet/log"
	gitignore "github.com/sabhiram/go-gitignore"
)

// Ignorer defines the interface for pattern matching.
type Ignorer interface {
	MatchesPath(path string) bool
}

// combinedIgnorer wraps two ignorers.
type combinedIgnorer struct {
	file     *gitignore.GitIgnore
	patterns *gitignore.GitIgnore
}

// MatchesPath returns true if the path matches any ignore pattern.
func (c *combinedIgnorer) MatchesPath(path string) bool {
	return c.file.MatchesPath(path) || c.patterns.MatchesPath(path)
}

// FileWalker implements Walker for traversing a file system.
type FileWalker struct {
	opts    WalkOptions
	ignorer Ignorer
	stats   WalkStats
	extSet  map[string]bool
}

// NewFileWalker creates a new file walker.
func NewFileWalker(opts WalkOptions) (*FileWalker, error) {
	// Ensure root is absolute
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve root path: %w", err)
	}
	opts.Root = root

	// Check root exists
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("root path does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", root)
	}

	w := &FileWalker{
		opts: opts,
	}

	// Build extension set for fast lookup
	if len(opts.Extensions) > 0 {
		w.extSet = make(map[string]bool)
		for _, ext := range opts.Extensions {
			// Normalize extension to have leading dot
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			w.extSet[strings.ToLower(ext)] = true
		}
	}

	// Initialize gitignore
	if err := w.initIgnorer(); err != nil {
		return nil, err
	}

	return w, nil
}

// initIgnorer initializes the gitignore matcher.
func (w *FileWalker) initIgnorer() error {
	var patterns []string

	// Add custom ignore patterns
	patterns = append(patterns, w.opts.IgnorePatterns...)

	// Add default patterns for binary and generated files
	patterns = append(patterns, defaultIgnorePatterns...)

	// Load .gitignore from root if it exists
	if w.opts.UseGitignore {
		gitignorePath := filepath.Join(w.opts.Root, ".gitignore")
		if _, err := os.Stat(gitignorePath); err == nil {
			gi, err := gitignore.CompileIgnoreFile(gitignorePath)
			if err != nil {
				log.Warn("Failed to parse .gitignore", "path", gitignorePath, "error", err)
			} else {
				// Combine with our patterns
				combined := gitignore.CompileIgnoreLines(patterns...)
				w.ignorer = &combinedIgnorer{
					file:     gi,
					patterns: combined,
				}
				return nil
			}
		}
	}

	// Use only our patterns
	w.ignorer = gitignore.CompileIgnoreLines(patterns...)
	return nil
}

// Walk traverses the directory tree.
func (w *FileWalker) Walk(fn func(FileInfo) error) error {
	w.stats = WalkStats{} // Reset stats

	return filepath.WalkDir(w.opts.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			log.Debug("Error accessing path", "path", path, "error", err)
			return nil // Skip errors, continue walking
		}

		// Get relative path for pattern matching
		relPath, err := filepath.Rel(w.opts.Root, path)
		if err != nil {
			relPath = path
		}

		// Check if we should skip this entry
		if d.IsDir() {
			if w.shouldSkipDir(d.Name(), relPath) {
				w.stats.DirsSkipped++
				return filepath.SkipDir
			}
			return nil
		}

		// Check max file count
		if w.opts.MaxFileCount > 0 && w.stats.FilesFound >= w.opts.MaxFileCount {
			return filepath.SkipAll
		}

		// Skip if file should be ignored
		if w.shouldSkipFile(d.Name(), relPath) {
			w.stats.FilesSkipped++
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			log.Debug("Failed to get file info", "path", path, "error", err)
			return nil
		}

		// Check file size
		if w.opts.MaxFileSize > 0 && info.Size() > w.opts.MaxFileSize {
			w.stats.FilesSkipped++
			w.stats.SkippedBytes += info.Size()
			return nil
		}

		// Check extension filter
		if w.extSet != nil {
			ext := strings.ToLower(filepath.Ext(path))
			if !w.extSet[ext] {
				w.stats.FilesSkipped++
				return nil
			}
		}

		// Check if file is binary
		if isBinary, err := isBinaryFile(path); err != nil || isBinary {
			w.stats.FilesSkipped++
			return nil
		}

		// Compute file hash
		hash, err := hashFile(path)
		if err != nil {
			log.Debug("Failed to hash file", "path", path, "error", err)
			return nil
		}

		// Detect language
		lang := DetectLanguage(path)

		fileInfo := FileInfo{
			Path:     path,
			RelPath:  relPath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Hash:     hash,
			Language: lang,
		}

		w.stats.FilesFound++
		w.stats.TotalBytes += info.Size()

		return fn(fileInfo)
	})
}

// Stats returns the walk statistics.
func (w *FileWalker) Stats() WalkStats {
	return w.stats
}

// shouldSkipDir checks if a directory should be skipped.
func (w *FileWalker) shouldSkipDir(name, relPath string) bool {
	// Always skip .git
	if name == ".git" {
		return true
	}

	// Skip hidden directories unless configured otherwise
	if !w.opts.IncludeHidden && strings.HasPrefix(name, ".") {
		return true
	}

	// Check gitignore patterns
	if w.ignorer != nil && w.ignorer.MatchesPath(relPath+"/") {
		return true
	}

	return false
}

// shouldSkipFile checks if a file should be skipped.
func (w *FileWalker) shouldSkipFile(name, relPath string) bool {
	// Skip hidden files unless configured otherwise
	if !w.opts.IncludeHidden && strings.HasPrefix(name, ".") {
		return true
	}

	// Check gitignore patterns
	if w.ignorer != nil && w.ignorer.MatchesPath(relPath) {
		return true
	}

	return false
}

// hashFile computes the xxhash of a file's contents.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%016x", h.Sum64()), nil
}

// HashContent computes the xxhash of content bytes.
func HashContent(content []byte) string {
	return fmt.Sprintf("%016x", xxhash.Sum64(content))
}

// isBinaryFile checks if a file appears to be binary.
func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read first 8KB
	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	return isBinaryContent(buf[:n]), nil
}

// isBinaryContent checks if content appears to be binary.
func isBinaryContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check for null bytes (strong indicator of binary)
	for _, b := range content {
		if b == 0 {
			return true
		}
	}

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range content {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider binary
	return float64(nonPrintable)/float64(len(content)) > 0.3
}

// Default patterns to ignore (common binary/generated files).
var defaultIgnorePatterns = []string{
	// Build outputs
	"node_modules/",
	"vendor/",
	"dist/",
	"build/",
	"out/",
	"target/",
	"bin/",
	"obj/",
	"*.min.js",
	"*.min.css",
	"*.bundle.js",

	// Package locks (often huge)
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"Cargo.lock",
	"poetry.lock",
	"go.sum",

	// IDE/editor
	".idea/",
	".vscode/",
	"*.swp",
	"*.swo",
	"*~",

	// OS files
	".DS_Store",
	"Thumbs.db",

	// Binary file extensions
	"*.exe",
	"*.dll",
	"*.so",
	"*.dylib",
	"*.a",
	"*.o",
	"*.obj",
	"*.pyc",
	"*.pyo",
	"*.class",
	"*.jar",
	"*.war",
	"*.ear",
	"*.zip",
	"*.tar",
	"*.gz",
	"*.bz2",
	"*.xz",
	"*.rar",
	"*.7z",
	"*.pdf",
	"*.doc",
	"*.docx",
	"*.xls",
	"*.xlsx",
	"*.ppt",
	"*.pptx",
	"*.png",
	"*.jpg",
	"*.jpeg",
	"*.gif",
	"*.bmp",
	"*.ico",
	"*.svg",
	"*.mp3",
	"*.mp4",
	"*.wav",
	"*.avi",
	"*.mov",
	"*.mkv",
	"*.woff",
	"*.woff2",
	"*.ttf",
	"*.eot",
	"*.otf",

	// Database files
	"*.db",
	"*.sqlite",
	"*.sqlite3",

	// Coverage and test artifacts
	"coverage/",
	".nyc_output/",
	"*.lcov",

	// Generated files
	"*.generated.*",
	"*.gen.*",
}
