package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeCodeConfigPath returns the path to the Claude Code config file.
func ClaudeCodeConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

// ClaudeCodeInstall installs lgrep into Claude Code.
func ClaudeCodeInstall() error {
	configPath := ClaudeCodeConfigPath()

	// Read existing config
	config := make(map[string]any)
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse existing config: %w", err)
		}
	}

	// Get or create mcpServers section
	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		mcpServers = make(map[string]any)
	}

	// Add lgrep server
	mcpServers["lgrep"] = map[string]any{
		"command": "lgrep",
		"args":    []string{"mcp"},
	}
	config["mcpServers"] = mcpServers

	// Write config
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("Successfully installed lgrep into Claude Code")
	fmt.Printf("Config updated: %s\n", configPath)
	printWarning("Claude Code")

	return nil
}

// ClaudeCodeUninstall removes lgrep from Claude Code.
func ClaudeCodeUninstall() error {
	configPath := ClaudeCodeConfigPath()

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Claude Code config not found, nothing to uninstall")
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	config := make(map[string]any)
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Remove lgrep from mcpServers
	if mcpServers, ok := config["mcpServers"].(map[string]any); ok {
		delete(mcpServers, "lgrep")
		config["mcpServers"] = mcpServers
	}

	// Write config
	data, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("Successfully uninstalled lgrep from Claude Code")

	return nil
}
