package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nickcecere/lgrep/internal/install"
)

// installCmd represents the install parent command.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install lgrep into AI coding agents",
	Long: `Install lgrep as an MCP server into AI coding agents.

Supported agents:
  - claude-code: Claude Code (Anthropic)
  - opencode: OpenCode
  - codex: Codex (OpenAI)

After installation, lgrep will automatically start when you begin a coding session
and provide semantic code search capabilities to the AI agent.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// installClaudeCodeCmd installs lgrep into Claude Code.
var installClaudeCodeCmd = &cobra.Command{
	Use:   "claude-code",
	Short: "Install lgrep into Claude Code",
	Long: `Install lgrep as an MCP server into Claude Code.

This adds an entry to ~/.claude.json that configures Claude Code to start
lgrep's MCP server when beginning a coding session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return install.ClaudeCodeInstall()
	},
}

// installOpencodeCmd installs lgrep into OpenCode.
var installOpencodeCmd = &cobra.Command{
	Use:   "opencode",
	Short: "Install lgrep into OpenCode",
	Long: `Install lgrep as an MCP server into OpenCode.

This:
  1. Creates a tool definition at ~/.config/opencode/tool/lgrep.ts
  2. Adds an MCP server entry to ~/.config/opencode/opencode.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return install.OpenCodeInstall()
	},
}

// installCodexCmd installs lgrep into Codex.
var installCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Install lgrep into Codex",
	Long: `Install lgrep as an MCP server into Codex.

This:
  1. Runs 'codex mcp add lgrep lgrep mcp' to register the MCP server
  2. Appends the lgrep skill definition to ~/.codex/AGENTS.md`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return install.CodexInstall()
	},
}

// uninstallCmd represents the uninstall parent command.
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall lgrep from AI coding agents",
	Long: `Uninstall lgrep from AI coding agents.

Supported agents:
  - claude-code: Claude Code (Anthropic)
  - opencode: OpenCode
  - codex: Codex (OpenAI)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// uninstallClaudeCodeCmd uninstalls lgrep from Claude Code.
var uninstallClaudeCodeCmd = &cobra.Command{
	Use:   "claude-code",
	Short: "Uninstall lgrep from Claude Code",
	RunE: func(cmd *cobra.Command, args []string) error {
		return install.ClaudeCodeUninstall()
	},
}

// uninstallOpencodeCmd uninstalls lgrep from OpenCode.
var uninstallOpencodeCmd = &cobra.Command{
	Use:   "opencode",
	Short: "Uninstall lgrep from OpenCode",
	RunE: func(cmd *cobra.Command, args []string) error {
		return install.OpenCodeUninstall()
	},
}

// uninstallCodexCmd uninstalls lgrep from Codex.
var uninstallCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Uninstall lgrep from Codex",
	RunE: func(cmd *cobra.Command, args []string) error {
		return install.CodexUninstall()
	},
}

// installAllCmd installs lgrep into all supported agents.
var installAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Install lgrep into all supported agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Installing lgrep into all supported agents...")
		fmt.Println()

		fmt.Println("=== Claude Code ===")
		if err := install.ClaudeCodeInstall(); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}
		fmt.Println()

		fmt.Println("=== OpenCode ===")
		if err := install.OpenCodeInstall(); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}
		fmt.Println()

		fmt.Println("=== Codex ===")
		if err := install.CodexInstall(); err != nil {
			fmt.Printf("Warning: %v\n", err)
		}

		return nil
	},
}

func init() {
	// Add install subcommands
	installCmd.AddCommand(installClaudeCodeCmd)
	installCmd.AddCommand(installOpencodeCmd)
	installCmd.AddCommand(installCodexCmd)
	installCmd.AddCommand(installAllCmd)

	// Add uninstall subcommands
	uninstallCmd.AddCommand(uninstallClaudeCodeCmd)
	uninstallCmd.AddCommand(uninstallOpencodeCmd)
	uninstallCmd.AddCommand(uninstallCodexCmd)
}
