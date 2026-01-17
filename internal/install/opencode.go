package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// OpenCodeConfigPath returns the path to the OpenCode config file.
func OpenCodeConfigPath() string {
	home, _ := os.UserHomeDir()

	// Check for both .json and .jsonc
	jsonPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	jsoncPath := filepath.Join(home, ".config", "opencode", "opencode.jsonc")

	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}
	if _, err := os.Stat(jsoncPath); err == nil {
		return jsoncPath
	}
	return jsonPath // Default to .json
}

// OpenCodeToolPath returns the path to the lgrep tool file for OpenCode.
func OpenCodeToolPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "tool", "lgrep.ts")
}

// OpenCodeInstall installs lgrep into OpenCode.
func OpenCodeInstall() error {
	// Create tool directory
	toolPath := OpenCodeToolPath()
	if err := os.MkdirAll(filepath.Dir(toolPath), 0755); err != nil {
		return fmt.Errorf("failed to create tool directory: %w", err)
	}

	// Write tool file
	if err := os.WriteFile(toolPath, []byte(OpenCodeToolDefinition), 0644); err != nil {
		return fmt.Errorf("failed to write tool file: %w", err)
	}
	fmt.Printf("Created tool file: %s\n", toolPath)

	// Update config
	configPath := OpenCodeConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing config
	config := make(map[string]any)
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse existing config: %w", err)
		}
	}

	// Add schema if not present
	if _, ok := config["$schema"]; !ok {
		config["$schema"] = "https://opencode.ai/config.json"
	}

	// Get or create mcp section
	mcpConfig, ok := config["mcp"].(map[string]any)
	if !ok {
		mcpConfig = make(map[string]any)
	}

	// Add lgrep server
	mcpConfig["lgrep"] = map[string]any{
		"type":    "local",
		"command": []string{"lgrep", "mcp"},
		"enabled": true,
	}
	config["mcp"] = mcpConfig

	// Write config
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("Successfully installed lgrep into OpenCode")
	fmt.Printf("Config updated: %s\n", configPath)
	printWarning("OpenCode")

	return nil
}

// OpenCodeUninstall removes lgrep from OpenCode.
func OpenCodeUninstall() error {
	// Remove tool file
	toolPath := OpenCodeToolPath()
	if err := os.Remove(toolPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove tool file: %w", err)
	}
	fmt.Printf("Removed tool file: %s\n", toolPath)

	// Update config
	configPath := OpenCodeConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("OpenCode config not found, nothing more to uninstall")
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	config := make(map[string]any)
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Remove lgrep from mcp section
	if mcpConfig, ok := config["mcp"].(map[string]any); ok {
		delete(mcpConfig, "lgrep")
		config["mcp"] = mcpConfig
	}

	// Write config
	data, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("Successfully uninstalled lgrep from OpenCode")

	return nil
}
