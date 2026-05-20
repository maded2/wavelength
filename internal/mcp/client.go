package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"wavelength/internal/config"
)

// Manager connects to external MCP servers and exposes their tools
// for use by the LLM during interview sessions.
type Manager struct {
	cfg      *config.MCPConfig
	sessions map[string]*mcp.ClientSession // keyed by server name
	tools    []*Tool                       // aggregated tools from all servers
	mu       sync.RWMutex
	connected bool
}

// Tool wraps an MCP tool with its server session for execution.
type Tool struct {
	// Name is the unique tool name (prefixed with server name to avoid collisions).
	Name string
	// Description is the human-readable tool description.
	Description string
	// InputSchema is the JSON Schema for the tool's parameters.
	InputSchema map[string]interface{}
	// ServerName is the name of the MCP server that provides this tool.
	ServerName string
	// ToolName is the original tool name from the MCP server.
	ToolName string
}

// New creates a new MCP manager with the given configuration.
func New(cfg *config.MCPConfig) *Manager {
	return &Manager{
		cfg:      cfg,
		sessions: make(map[string]*mcp.ClientSession),
	}
}

// Connect establishes connections to all configured MCP servers and discovers
// their available tools. Call this once at startup.
func (m *Manager) Connect(ctx context.Context) error {
	if len(m.cfg.Servers) == 0 {
		log.Println("[MCP] No MCP servers configured, skipping connection")
		return nil
	}

	log.Printf("[MCP] Connecting to %d MCP server(s)...", len(m.cfg.Servers))

	for _, srv := range m.cfg.Servers {
		timeout := time.Duration(srv.Timeout) * time.Second
		if timeout <= 0 {
			timeout = 10 * time.Second
		}

		connectCtx, cancel := context.WithTimeout(ctx, timeout)
		err := m.connectServer(connectCtx, srv)
		cancel()

		if err != nil {
			log.Printf("[MCP] WARNING: Failed to connect to MCP server %q: %v", srv.Name, err)
			continue
		}

		log.Printf("[MCP] Connected to MCP server %q (%s transport)", srv.Name, srv.Transport)
	}

	// Discover tools from all connected servers
	if err := m.discoverTools(ctx); err != nil {
		log.Printf("[MCP] WARNING: Failed to discover tools: %v", err)
	}

	m.mu.Lock()
	m.connected = true
	m.mu.Unlock()

	toolCount := len(m.tools)
	log.Printf("[MCP] MCP initialization complete: %d tool(s) available from connected servers", toolCount)
	return nil
}

// connectServer connects to a single MCP server and stores the session.
func (m *Manager) connectServer(ctx context.Context, srv config.MCPServerConfig) error {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "wavelength",
		Version: "1.0.0",
	}, nil)

	var transport mcp.Transport
	switch srv.Transport {
	case "stdio":
		if srv.Command == "" {
			return fmt.Errorf("MCP server %q: command is required for stdio transport", srv.Name)
		}
		cmd := exec.CommandContext(ctx, srv.Command, srv.Args...)
		cmd.Env = os.Environ()
		for k, v := range srv.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
		transport = &mcp.CommandTransport{Command: cmd}

	case "sse":
		if srv.URL == "" {
			return fmt.Errorf("MCP server %q: url is required for SSE transport", srv.Name)
		}
		transport = &mcp.SSEClientTransport{Endpoint: srv.URL}

	default:
		return fmt.Errorf("MCP server %q: unsupported transport %q (supported: stdio, sse)", srv.Name, srv.Transport)
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	m.mu.Lock()
	m.sessions[srv.Name] = session
	m.mu.Unlock()

	return nil
}

// discoverTools queries all connected servers for their available tools
// and aggregates them into a unified list.
func (m *Manager) discoverTools(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tools = nil

	for name, session := range m.sessions {
		result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			log.Printf("[MCP] WARNING: Failed to list tools from server %q: %v", name, err)
			continue
		}

		for _, tool := range result.Tools {
			inputSchema := make(map[string]interface{})
			if tool.InputSchema != nil {
				if schema, ok := tool.InputSchema.(map[string]interface{}); ok {
					inputSchema = schema
				}
			}

			// Prefix tool name with server name to avoid collisions
			qualifiedName := fmt.Sprintf("mcp::%s::%s", name, tool.Name)

			m.tools = append(m.tools, &Tool{
				Name:        qualifiedName,
				Description: tool.Description,
				InputSchema: inputSchema,
				ServerName:  name,
				ToolName:    tool.Name,
			})
		}

		log.Printf("[MCP] Server %q: discovered %d tool(s)", name, len(result.Tools))
	}

	return nil
}

// Tools returns all discovered MCP tools.
func (m *Manager) Tools() []*Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid concurrent modification
	result := make([]*Tool, len(m.tools))
	copy(result, m.tools)
	return result
}

// CallTool executes a tool call on the appropriate MCP server.
// toolName is the qualified tool name (mcp::<server>::<tool>).
// args is a map of argument name to value.
func (m *Manager) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	m.mu.RLock()
	session, ok := m.sessions[extractServerName(toolName)]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("MCP server not connected: %s", extractServerName(toolName))
	}

	toolNameFromQualified := extractToolName(toolName)

	params := &mcp.CallToolParams{
		Name:      toolNameFromQualified,
		Arguments: args,
	}

	result, err := session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from result
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}

	if result.IsError {
		return fmt.Sprintf("[MCP error from %s]: %s", toolNameFromQualified, joinStrings(parts, "\n")), fmt.Errorf("MCP tool returned error")
	}

	return joinStrings(parts, "\n"), nil
}

// Close closes all MCP server connections.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string
	for name, session := range m.sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing MCP sessions: %s", joinStrings(errs, "; "))
	}

	log.Println("[MCP] All MCP connections closed")
	return nil
}

// IsConnected returns true if at least one MCP server is connected.
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected && len(m.sessions) > 0
}

// extractServerName extracts the server name from a qualified tool name.
// Format: mcp::<server>::<tool>
func extractServerName(qualifiedName string) string {
	prefix := "mcp::"
	if !strings.HasPrefix(qualifiedName, prefix) {
		return ""
	}
	withoutPrefix := qualifiedName[len(prefix):]
	idx := strings.Index(withoutPrefix, "::")
	if idx == -1 {
		return withoutPrefix
	}
	return withoutPrefix[:idx]
}

// extractToolName extracts the original tool name from a qualified tool name.
// Format: mcp::<server>::<tool>
func extractToolName(qualifiedName string) string {
	prefix := "mcp::"
	if !strings.HasPrefix(qualifiedName, prefix) {
		return ""
	}
	withoutPrefix := qualifiedName[len(prefix):]
	idx := strings.Index(withoutPrefix, "::")
	if idx == -1 {
		return withoutPrefix
	}
	return withoutPrefix[idx+2:]
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}
