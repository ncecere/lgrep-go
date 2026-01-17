package fs

import (
	"path/filepath"
	"strings"
)

// Language constants for common programming languages.
const (
	LangGo         = "go"
	LangTypeScript = "typescript"
	LangJavaScript = "javascript"
	LangPython     = "python"
	LangRust       = "rust"
	LangJava       = "java"
	LangC          = "c"
	LangCPP        = "cpp"
	LangCSharp     = "csharp"
	LangRuby       = "ruby"
	LangPHP        = "php"
	LangSwift      = "swift"
	LangKotlin     = "kotlin"
	LangScala      = "scala"
	LangShell      = "shell"
	LangSQL        = "sql"
	LangHTML       = "html"
	LangCSS        = "css"
	LangJSON       = "json"
	LangYAML       = "yaml"
	LangTOML       = "toml"
	LangMarkdown   = "markdown"
	LangXML        = "xml"
	LangText       = "text"
	LangUnknown    = ""
)

// Language detection maps.
var (
	// extToLang maps file extensions to languages.
	extToLang = map[string]string{
		// Go
		".go": LangGo,

		// TypeScript/JavaScript
		".ts":  LangTypeScript,
		".tsx": LangTypeScript,
		".mts": LangTypeScript,
		".cts": LangTypeScript,
		".js":  LangJavaScript,
		".jsx": LangJavaScript,
		".mjs": LangJavaScript,
		".cjs": LangJavaScript,

		// Python
		".py":  LangPython,
		".pyi": LangPython,
		".pyw": LangPython,

		// Rust
		".rs": LangRust,

		// Java
		".java": LangJava,

		// C/C++
		".c":   LangC,
		".h":   LangC,
		".cc":  LangCPP,
		".cpp": LangCPP,
		".cxx": LangCPP,
		".hpp": LangCPP,
		".hxx": LangCPP,

		// C#
		".cs": LangCSharp,

		// Ruby
		".rb":   LangRuby,
		".rake": LangRuby,

		// PHP
		".php": LangPHP,

		// Swift
		".swift": LangSwift,

		// Kotlin
		".kt":  LangKotlin,
		".kts": LangKotlin,

		// Scala
		".scala": LangScala,

		// Shell
		".sh":   LangShell,
		".bash": LangShell,
		".zsh":  LangShell,
		".fish": LangShell,

		// SQL
		".sql": LangSQL,

		// Web
		".html": LangHTML,
		".htm":  LangHTML,
		".css":  LangCSS,
		".scss": LangCSS,
		".sass": LangCSS,
		".less": LangCSS,

		// Data formats
		".json":  LangJSON,
		".jsonc": LangJSON,
		".yaml":  LangYAML,
		".yml":   LangYAML,
		".toml":  LangTOML,
		".xml":   LangXML,

		// Documentation
		".md":       LangMarkdown,
		".markdown": LangMarkdown,
		".txt":      LangText,
		".text":     LangText,
		".rst":      LangText,
	}

	// filenameToLang maps specific filenames to languages.
	filenameToLang = map[string]string{
		"Makefile":      LangShell,
		"makefile":      LangShell,
		"Dockerfile":    LangShell,
		"dockerfile":    LangShell,
		"Rakefile":      LangRuby,
		"Gemfile":       LangRuby,
		"Jenkinsfile":   LangShell,
		".bashrc":       LangShell,
		".zshrc":        LangShell,
		".profile":      LangShell,
		".gitignore":    LangText,
		".gitconfig":    LangText,
		".editorconfig": LangText,
	}
)

// DetectLanguage determines the programming language of a file based on its path.
func DetectLanguage(path string) string {
	filename := filepath.Base(path)

	// Check specific filenames first
	if lang, ok := filenameToLang[filename]; ok {
		return lang
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := extToLang[ext]; ok {
		return lang
	}

	return LangUnknown
}

// IsCodeFile returns true if the file appears to be source code.
func IsCodeFile(path string) bool {
	lang := DetectLanguage(path)
	switch lang {
	case LangGo, LangTypeScript, LangJavaScript, LangPython, LangRust,
		LangJava, LangC, LangCPP, LangCSharp, LangRuby, LangPHP,
		LangSwift, LangKotlin, LangScala, LangShell, LangSQL:
		return true
	default:
		return false
	}
}

// SupportsCodeChunking returns true if the language supports code-aware chunking.
func SupportsCodeChunking(lang string) bool {
	switch lang {
	case LangGo, LangTypeScript, LangJavaScript, LangPython, LangRust,
		LangJava, LangC, LangCPP, LangCSharp, LangRuby, LangPHP,
		LangSwift, LangKotlin, LangScala:
		return true
	default:
		return false
	}
}
