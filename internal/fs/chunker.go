package fs

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"
)

// TextChunker implements basic text chunking with overlap.
type TextChunker struct {
	opts ChunkOptions
}

// NewTextChunker creates a new text chunker.
func NewTextChunker(opts ChunkOptions) *TextChunker {
	// Apply defaults for zero values
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = DefaultChunkOptions().ChunkSize
	}
	if opts.ChunkOverlap < 0 {
		opts.ChunkOverlap = DefaultChunkOptions().ChunkOverlap
	}
	if opts.MinChunkSize <= 0 {
		opts.MinChunkSize = DefaultChunkOptions().MinChunkSize
	}

	return &TextChunker{opts: opts}
}

// Chunk splits content into chunks.
func (c *TextChunker) Chunk(content string, filename string) []Chunk {
	if len(content) == 0 {
		return nil
	}

	// Check if we should use code-aware chunking
	lang := DetectLanguage(filename)
	if SupportsCodeChunking(lang) {
		return c.chunkCode(content, lang)
	}

	return c.chunkText(content)
}

// ChunkReader reads content from a reader and chunks it.
func (c *TextChunker) ChunkReader(r io.Reader, filename string) ([]Chunk, error) {
	var content strings.Builder
	scanner := bufio.NewScanner(r)
	// Increase buffer size for large lines
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return c.Chunk(content.String(), filename), nil
}

// chunkText performs simple text chunking with overlap.
func (c *TextChunker) chunkText(content string) []Chunk {
	var chunks []Chunk
	lines := strings.Split(content, "\n")

	chunkStart := 0
	chunkStartChar := 0
	currentSize := 0
	var currentLines []string

	for lineNum, line := range lines {
		lineLen := utf8.RuneCountInString(line) + 1 // +1 for newline

		// Check if adding this line would exceed chunk size
		if currentSize+lineLen > c.opts.ChunkSize && len(currentLines) > 0 {
			// Create chunk from current lines
			chunks = append(chunks, Chunk{
				Content:    strings.Join(currentLines, "\n"),
				StartLine:  chunkStart + 1, // 1-indexed
				EndLine:    chunkStart + len(currentLines),
				StartChar:  chunkStartChar,
				EndChar:    chunkStartChar + currentSize - 1,
				ChunkIndex: len(chunks),
			})

			// Calculate overlap
			overlapLines, overlapSize := c.calculateOverlap(currentLines)

			// Start new chunk with overlap
			currentLines = make([]string, len(overlapLines))
			copy(currentLines, overlapLines)
			chunkStart = lineNum - len(overlapLines)
			chunkStartChar = chunkStartChar + currentSize - overlapSize
			currentSize = overlapSize
		}

		currentLines = append(currentLines, line)
		currentSize += lineLen
	}

	// Add final chunk
	if len(currentLines) > 0 {
		content := strings.Join(currentLines, "\n")
		// Skip chunks that are too small (unless it's the only chunk)
		if len(chunks) == 0 || utf8.RuneCountInString(content) >= c.opts.MinChunkSize {
			chunks = append(chunks, Chunk{
				Content:    content,
				StartLine:  chunkStart + 1,
				EndLine:    chunkStart + len(currentLines),
				StartChar:  chunkStartChar,
				EndChar:    chunkStartChar + currentSize - 1,
				ChunkIndex: len(chunks),
			})
		} else if len(chunks) > 0 {
			// Merge with previous chunk
			prev := &chunks[len(chunks)-1]
			prev.Content += "\n" + content
			prev.EndLine = chunkStart + len(currentLines)
			prev.EndChar = chunkStartChar + currentSize - 1
		}
	}

	return chunks
}

// calculateOverlap determines how many lines to include in the overlap.
func (c *TextChunker) calculateOverlap(lines []string) ([]string, int) {
	if c.opts.ChunkOverlap <= 0 || len(lines) == 0 {
		return nil, 0
	}

	// Work backwards from the end to find overlap
	var overlapLines []string
	overlapSize := 0

	for i := len(lines) - 1; i >= 0 && overlapSize < c.opts.ChunkOverlap; i-- {
		lineLen := utf8.RuneCountInString(lines[i]) + 1
		overlapLines = append([]string{lines[i]}, overlapLines...)
		overlapSize += lineLen
	}

	return overlapLines, overlapSize
}

// chunkCode performs code-aware chunking.
func (c *TextChunker) chunkCode(content string, lang string) []Chunk {
	lines := strings.Split(content, "\n")

	// Find function/class boundaries
	boundaries := findCodeBoundaries(lines, lang)

	if len(boundaries) == 0 {
		// Fall back to text chunking
		return c.chunkText(content)
	}

	var chunks []Chunk
	charOffset := 0

	for i, boundary := range boundaries {
		endLine := len(lines)
		if i+1 < len(boundaries) {
			endLine = boundaries[i+1]
		}

		// Build chunk content
		chunkLines := lines[boundary:endLine]
		chunkContent := strings.Join(chunkLines, "\n")
		chunkLen := utf8.RuneCountInString(chunkContent)

		// If chunk is too large, split it
		if chunkLen > c.opts.ChunkSize*2 {
			subChunks := c.chunkText(chunkContent)
			for _, sub := range subChunks {
				sub.StartLine += boundary
				sub.EndLine += boundary
				sub.StartChar += charOffset
				sub.EndChar += charOffset
				sub.ChunkIndex = len(chunks)
				chunks = append(chunks, sub)
			}
		} else if chunkLen >= c.opts.MinChunkSize {
			chunks = append(chunks, Chunk{
				Content:    chunkContent,
				StartLine:  boundary + 1,
				EndLine:    endLine,
				StartChar:  charOffset,
				EndChar:    charOffset + chunkLen,
				ChunkIndex: len(chunks),
			})
		}

		// Update character offset
		for j := boundary; j < endLine && j < len(lines); j++ {
			charOffset += utf8.RuneCountInString(lines[j]) + 1
		}
	}

	// If no chunks were created, fall back to text chunking
	if len(chunks) == 0 {
		return c.chunkText(content)
	}

	return chunks
}

// findCodeBoundaries finds line numbers where code blocks start.
func findCodeBoundaries(lines []string, lang string) []int {
	var boundaries []int
	inMultilineComment := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" {
			continue
		}

		// Handle multiline comments (simplified)
		if strings.Contains(trimmed, "/*") {
			inMultilineComment = true
		}
		if strings.Contains(trimmed, "*/") {
			inMultilineComment = false
			continue
		}
		if inMultilineComment {
			continue
		}

		// Check for function/class definitions
		if isDefinitionStart(trimmed, lang) {
			boundaries = append(boundaries, i)
		}
	}

	// If we found no boundaries, add line 0
	if len(boundaries) == 0 {
		boundaries = append(boundaries, 0)
	} else if boundaries[0] > 0 {
		// Include any content before the first boundary
		boundaries = append([]int{0}, boundaries...)
	}

	return boundaries
}

// isDefinitionStart checks if a line starts a function/class/type definition.
func isDefinitionStart(line, lang string) bool {
	switch lang {
	case LangGo:
		return strings.HasPrefix(line, "func ") ||
			strings.HasPrefix(line, "type ") ||
			strings.HasPrefix(line, "const ") ||
			strings.HasPrefix(line, "var ")

	case LangTypeScript, LangJavaScript:
		return strings.HasPrefix(line, "function ") ||
			strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "interface ") ||
			strings.HasPrefix(line, "type ") ||
			strings.HasPrefix(line, "const ") ||
			strings.HasPrefix(line, "let ") ||
			strings.HasPrefix(line, "export function ") ||
			strings.HasPrefix(line, "export class ") ||
			strings.HasPrefix(line, "export interface ") ||
			strings.HasPrefix(line, "export type ") ||
			strings.HasPrefix(line, "export const ") ||
			strings.HasPrefix(line, "export default ")

	case LangPython:
		return strings.HasPrefix(line, "def ") ||
			strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "async def ")

	case LangRust:
		return strings.HasPrefix(line, "fn ") ||
			strings.HasPrefix(line, "pub fn ") ||
			strings.HasPrefix(line, "struct ") ||
			strings.HasPrefix(line, "pub struct ") ||
			strings.HasPrefix(line, "enum ") ||
			strings.HasPrefix(line, "pub enum ") ||
			strings.HasPrefix(line, "impl ") ||
			strings.HasPrefix(line, "trait ") ||
			strings.HasPrefix(line, "pub trait ")

	case LangJava:
		return strings.Contains(line, "class ") ||
			strings.Contains(line, "interface ") ||
			strings.Contains(line, "enum ") ||
			(strings.Contains(line, "(") && strings.Contains(line, ")") &&
				(strings.Contains(line, "public ") || strings.Contains(line, "private ") ||
					strings.Contains(line, "protected ") || strings.Contains(line, "static ")))

	case LangC, LangCPP:
		// Simple heuristic: line with parentheses that doesn't end with semicolon
		return (strings.Contains(line, "(") && !strings.HasSuffix(line, ";") &&
			!strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#")) ||
			strings.HasPrefix(line, "struct ") ||
			strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "namespace ")

	case LangCSharp:
		return strings.Contains(line, "class ") ||
			strings.Contains(line, "interface ") ||
			strings.Contains(line, "struct ") ||
			strings.Contains(line, "enum ") ||
			strings.Contains(line, "namespace ")

	case LangRuby:
		return strings.HasPrefix(line, "def ") ||
			strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "module ")

	case LangPHP:
		return strings.HasPrefix(line, "function ") ||
			strings.Contains(line, "class ") ||
			strings.Contains(line, "interface ") ||
			strings.Contains(line, "trait ")

	case LangSwift:
		return strings.HasPrefix(line, "func ") ||
			strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "struct ") ||
			strings.HasPrefix(line, "enum ") ||
			strings.HasPrefix(line, "protocol ") ||
			strings.HasPrefix(line, "extension ")

	case LangKotlin:
		return strings.HasPrefix(line, "fun ") ||
			strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "interface ") ||
			strings.HasPrefix(line, "object ") ||
			strings.HasPrefix(line, "data class ")

	case LangScala:
		return strings.HasPrefix(line, "def ") ||
			strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "object ") ||
			strings.HasPrefix(line, "trait ") ||
			strings.HasPrefix(line, "case class ")
	}

	return false
}
