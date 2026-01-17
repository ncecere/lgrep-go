package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/nickcecere/lgrep/internal/config"
	"github.com/nickcecere/lgrep/internal/embeddings"
	"github.com/nickcecere/lgrep/internal/indexer"
	"github.com/nickcecere/lgrep/internal/search"
	"github.com/nickcecere/lgrep/internal/store"
)

const (
	// MCPVersion is the protocol version we support.
	MCPVersion = "2024-11-05"

	// ServerName is the name of this MCP server.
	ServerName = "lgrep"

	// ServerVersion is the version of this server.
	ServerVersion = "1.0.0"
)

// Server is the MCP server for lgrep.
type Server struct {
	store    store.Store
	embedder embeddings.Service
	searcher *search.Searcher
	indexer  *indexer.Indexer
	cfg      *config.Config

	// Stdin/stdout for communication
	reader *bufio.Reader
	writer io.Writer

	// State
	initialized bool
}

// NewServer creates a new MCP server.
func NewServer(st store.Store, emb embeddings.Service, cfg *config.Config) *Server {
	return &Server{
		store:    st,
		embedder: emb,
		searcher: search.New(st, emb),
		indexer:  indexer.New(st, emb, cfg),
		cfg:      cfg,
		reader:   bufio.NewReader(os.Stdin),
		writer:   os.Stdout,
	}
}

// Run starts the MCP server and processes requests until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	log.Info("MCP server starting")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read a line from stdin
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				log.Info("MCP server received EOF, shutting down")
				return nil
			}
			log.Error("Failed to read from stdin", "error", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse the request
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(nil, ErrorCodeParse, "Parse error", err.Error())
			continue
		}

		// Handle the request
		s.handleRequest(ctx, req)
	}
}

// handleRequest processes a single MCP request.
func (s *Server) handleRequest(ctx context.Context, req Request) {
	log.Debug("Received request", "method", req.Method, "id", req.ID)

	var result any
	var err error

	switch req.Method {
	case "initialize":
		result, err = s.handleInitialize(req.Params)
	case "initialized":
		// This is a notification, no response needed
		s.initialized = true
		log.Info("MCP server initialized")
		return
	case "tools/list":
		result, err = s.handleListTools()
	case "tools/call":
		result, err = s.handleCallTool(ctx, req.Params)
	case "ping":
		result = map[string]any{}
	default:
		s.sendError(req.ID, ErrorCodeMethodNotFound, "Method not found", req.Method)
		return
	}

	if err != nil {
		s.sendError(req.ID, ErrorCodeInternal, "Internal error", err.Error())
		return
	}

	s.sendResult(req.ID, result)
}

// handleInitialize handles the initialize request.
func (s *Server) handleInitialize(params json.RawMessage) (*InitializeResult, error) {
	var p InitializeParams
	if params != nil {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	log.Info("Initializing MCP server",
		"clientName", p.ClientInfo.Name,
		"clientVersion", p.ClientInfo.Version,
		"protocolVersion", p.ProtocolVersion,
	)

	return &InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    ServerName,
			Version: ServerVersion,
		},
	}, nil
}

// handleListTools returns the list of available tools.
func (s *Server) handleListTools() (*ListToolsResult, error) {
	tools := []Tool{
		{
			Name:        "lgrep_search",
			Description: "Semantic code search. Find relevant code using natural language queries.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "The search query in natural language",
					},
					"path": {
						Type:        "string",
						Description: "Directory path to search in (default: current directory)",
						Default:     ".",
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of results to return",
						Default:     10,
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "lgrep_index",
			Description: "Index a directory for semantic search. Run this before searching a new project.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Directory path to index",
						Default:     ".",
					},
				},
			},
		},
	}

	return &ListToolsResult{Tools: tools}, nil
}

// handleCallTool executes a tool and returns the result.
func (s *Server) handleCallTool(ctx context.Context, params json.RawMessage) (*CallToolResult, error) {
	var p CallToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	log.Debug("Calling tool", "name", p.Name, "arguments", p.Arguments)

	var resultText string
	var isError bool

	switch p.Name {
	case "lgrep_search":
		resultText, isError = s.toolSearch(ctx, p.Arguments)
	case "lgrep_index":
		resultText, isError = s.toolIndex(ctx, p.Arguments)
	default:
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", p.Name)}},
			IsError: true,
		}, nil
	}

	return &CallToolResult{
		Content: []ContentBlock{{Type: "text", Text: resultText}},
		IsError: isError,
	}, nil
}

// toolSearch performs a semantic search.
func (s *Server) toolSearch(ctx context.Context, args map[string]any) (string, bool) {
	query, _ := args["query"].(string)
	if query == "" {
		return "Error: query is required", true
	}

	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := args["limit"].(string); ok {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Sprintf("Error: failed to resolve path: %v", err), true
	}

	// Determine store name
	storeName := filepath.Base(absPath)

	// Check if store exists, auto-index if not
	storeRecord, _ := s.store.GetStore(storeName)
	if storeRecord == nil {
		// Auto-index
		opts := indexer.IndexOptions{
			StoreName: storeName,
			Path:      absPath,
			Force:     false,
			BatchSize: 50,
		}
		if err := s.indexer.Index(ctx, opts); err != nil {
			return fmt.Sprintf("Error: failed to index: %v", err), true
		}
	}

	// Perform search
	opts := search.SearchOptions{
		StoreName:      storeName,
		TopK:           limit,
		MinScore:       0.0,
		IncludeContent: true,
	}

	results, err := s.searcher.Search(ctx, query, opts)
	if err != nil {
		return fmt.Sprintf("Error: search failed: %v", err), true
	}

	if len(results) == 0 {
		return "No results found.", false
	}

	// Format results
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] %s (lines %d-%d) - %.1f%% match\n",
			i+1, r.RelativePath, r.StartLine, r.EndLine, r.Score*100))
		if r.Content != "" {
			// Truncate content if too long
			content := r.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			sb.WriteString(content)
			sb.WriteString("\n\n")
		}
	}

	return sb.String(), false
}

// toolIndex indexes a directory.
func (s *Server) toolIndex(ctx context.Context, args map[string]any) (string, bool) {
	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Sprintf("Error: failed to resolve path: %v", err), true
	}

	storeName := filepath.Base(absPath)

	opts := indexer.IndexOptions{
		StoreName: storeName,
		Path:      absPath,
		Force:     false,
		BatchSize: 50,
	}

	if err := s.indexer.Index(ctx, opts); err != nil {
		return fmt.Sprintf("Error: indexing failed: %v", err), true
	}

	// Get stats
	storeRecord, _ := s.store.GetStore(storeName)
	if storeRecord != nil {
		stats, _ := s.store.GetStats(storeRecord.ID)
		if stats != nil {
			return fmt.Sprintf("Successfully indexed %s: %d files, %d chunks",
				absPath, stats.FileCount, stats.ChunkCount), false
		}
	}

	return fmt.Sprintf("Successfully indexed %s", absPath), false
}

// sendResult sends a successful response.
func (s *Server) sendResult(id any, result any) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.send(resp)
}

// sendError sends an error response.
func (s *Server) sendError(id any, code int, message, data string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.send(resp)
}

// send writes a response to stdout.
func (s *Server) send(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error("Failed to marshal response", "error", err)
		return
	}
	fmt.Fprintln(s.writer, string(data))
}
