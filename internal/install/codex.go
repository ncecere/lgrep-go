package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CodexAgentsPath returns the path to the Codex AGENTS.md file.
func CodexAgentsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "AGENTS.md")
}

// CodexInstall installs lgrep into Codex.
func CodexInstall() error {
	// Add MCP server using codex CLI
	cmd := exec.Command("codex", "mcp", "add", "lgrep", "lgrep", "mcp")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add MCP server (is codex installed?): %w", err)
	}
	fmt.Println("Added lgrep MCP server to Codex")

	// Append skill to AGENTS.md
	agentsPath := CodexAgentsPath()
	if err := os.MkdirAll(filepath.Dir(agentsPath), 0755); err != nil {
		return fmt.Errorf("failed to create .codex directory: %w", err)
	}

	// Read existing content
	existingContent := ""
	if data, err := os.ReadFile(agentsPath); err == nil {
		existingContent = string(data)
	}

	// Check if skill is already present
	if strings.Contains(existingContent, "name: lgrep") {
		fmt.Println("lgrep skill already present in AGENTS.md")
	} else {
		// Append skill
		f, err := os.OpenFile(agentsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open AGENTS.md: %w", err)
		}
		defer f.Close()

		if _, err := f.WriteString(SkillMarkdown); err != nil {
			return fmt.Errorf("failed to write skill to AGENTS.md: %w", err)
		}
		fmt.Printf("Added lgrep skill to: %s\n", agentsPath)
	}

	fmt.Println("Successfully installed lgrep into Codex")
	printWarning("Codex")

	return nil
}

// CodexUninstall removes lgrep from Codex.
func CodexUninstall() error {
	// Remove MCP server using codex CLI
	cmd := exec.Command("codex", "mcp", "remove", "lgrep")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Note: failed to remove MCP server: %v\n", err)
	} else {
		fmt.Println("Removed lgrep MCP server from Codex")
	}

	// Remove skill from AGENTS.md
	agentsPath := CodexAgentsPath()
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Codex AGENTS.md not found, nothing more to uninstall")
			return nil
		}
		return fmt.Errorf("failed to read AGENTS.md: %w", err)
	}

	content := string(data)

	// Remove the skill markdown
	newContent := strings.Replace(content, SkillMarkdown, "", -1)
	newContent = strings.Replace(newContent, strings.TrimSpace(SkillMarkdown), "", -1)

	// Clean up multiple newlines
	for strings.Contains(newContent, "\n\n\n") {
		newContent = strings.Replace(newContent, "\n\n\n", "\n\n", -1)
	}
	newContent = strings.TrimSpace(newContent)

	if newContent == "" {
		// Remove the file if it's empty
		if err := os.Remove(agentsPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove empty AGENTS.md: %w", err)
		}
		fmt.Println("Removed empty AGENTS.md")
	} else {
		// Write the updated content
		if err := os.WriteFile(agentsPath, []byte(newContent+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write AGENTS.md: %w", err)
		}
		fmt.Println("Removed lgrep skill from AGENTS.md")
	}

	fmt.Println("Successfully uninstalled lgrep from Codex")

	return nil
}
