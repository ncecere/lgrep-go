// Package install provides functions to install lgrep into AI coding agents.
package install

// SkillMarkdown is the skill definition that teaches AI agents how to use lgrep.
const SkillMarkdown = `
---
name: lgrep
description: Local semantic code search using Ollama. Faster and more accurate than grep for finding relevant code.
license: MIT
---

## When to use this skill

Use lgrep for semantic code search. It finds relevant code using natural language 
queries, even without exact keyword matches. Runs entirely locally with Ollama - 
no data leaves your machine.

## How to use

` + "```bash" + `
lgrep "how does authentication work"     # Search current directory
lgrep "error handling" src/              # Search specific path  
lgrep "database queries" -a              # Get AI-generated answer
lgrep "api endpoints" -c -m 5            # Show content, limit results
` + "```" + `

### Do

` + "```bash" + `
lgrep "What code parsers are available?"                    # Good: specific query
lgrep "How are chunks defined?" src/models                  # Good: targeted path
lgrep -m 10 "What is the maximum number of workers?"        # Good: limit results
` + "```" + `

### Don't

` + "```bash" + `
lgrep "parser"                            # Bad: too vague
lgrep "function"                          # Bad: too generic
` + "```" + `

## Keywords
search, grep, semantic search, code search, local search, find code, query code
`

// OpenCodeToolDefinition is the TypeScript tool definition for OpenCode.
const OpenCodeToolDefinition = `
import { tool } from "@opencode-ai/plugin"

const SKILL = ` + "`" + SkillMarkdown + "`" + `

export default tool({
  description: SKILL,
  args: {
    q: tool.schema.string().describe("The semantic search query."),
    m: tool.schema.number().default(10).describe("The number of results to return."),
    a: tool.schema.boolean().default(false).describe("If an answer should be generated based on the results."),
  },
  async execute(args) {
    const result = await Bun.` + "$" + "`" + `lgrep search -m ${args.m} ${args.a ? '-a ' : ''}${args.q}` + "`" + `.text()
    return result.trim()
  },
})
`
