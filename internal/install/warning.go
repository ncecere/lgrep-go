package install

import "fmt"

// printWarning prints a warning about the background sync behavior.
func printWarning(agentName string) {
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  ⚠️  BACKGROUND SYNC ENABLED")
	fmt.Println()
	fmt.Println("  lgrep runs a background process that indexes your files to enable")
	fmt.Println("  semantic search. This process:")
	fmt.Println()
	fmt.Println("    • Starts automatically when you begin a session")
	fmt.Println("    • Indexes files in your working directory")
	fmt.Println("    • All data stays local (uses Ollama for embeddings)")
	fmt.Println("    • Stops when your session ends")
	fmt.Println()
	fmt.Printf("  To uninstall lgrep from %s:\n", agentName)
	fmt.Println()
	fmt.Printf("    lgrep uninstall %s\n", getUninstallTarget(agentName))
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════")
	fmt.Println()
}

// getUninstallTarget returns the uninstall command target for an agent.
func getUninstallTarget(agentName string) string {
	switch agentName {
	case "Claude Code":
		return "claude-code"
	case "OpenCode":
		return "opencode"
	case "Codex":
		return "codex"
	default:
		return agentName
	}
}
