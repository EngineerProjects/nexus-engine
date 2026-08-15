package mcp

import (
	"context"

	internalmcp "github.com/KPO-Tech/seshat/internal/tools/system/mcp"
)

type (
	ConfigScope           = internalmcp.ConfigScope
	ConnectServerConfig   = internalmcp.ConnectServerConfig
	IntegrationOptions    = internalmcp.IntegrationOptions
	IntegrationResult     = internalmcp.IntegrationResult
	Manager               = internalmcp.MCPClientManager
	McpJsonConfig         = internalmcp.McpJsonConfig
	McpServerConfig       = internalmcp.McpServerConfig
	McpServerType         = internalmcp.McpServerType
	ScopedMcpServerConfig = internalmcp.ScopedMcpServerConfig
	ServerConfig          = internalmcp.ServerConfig
	TransportType         = internalmcp.TransportType
	ValidationError       = internalmcp.ValidationError
)

const (
	ScopeProject    = internalmcp.ScopeProject
	ScopeUser       = internalmcp.ScopeUser
	ScopeLocal      = internalmcp.ScopeLocal
	ScopeEnterprise = internalmcp.ScopeEnterprise

	ServerTypeStdio     = internalmcp.ServerTypeStdio
	ServerTypeHTTP      = internalmcp.ServerTypeHTTP
	ServerTypeSSE       = internalmcp.ServerTypeSSE
	ServerTypeWebSocket = internalmcp.ServerTypeWebSocket
	ServerTypeSDK       = internalmcp.ServerTypeSDK

	// TransportType* values are for ServerConfig.Transport specifically
	// (the low-level Client's transport selector) - a separate type from
	// ServerType* above (McpServerType, used by the higher-level DB-facing
	// McpServerConfig), even though the string values overlap.
	TransportTypeStdio     = internalmcp.TransportTypeStdio
	TransportTypeHTTP      = internalmcp.TransportTypeHTTP
	TransportTypeSSE       = internalmcp.TransportTypeSSE
	TransportTypeWebSocket = internalmcp.TransportTypeWebSocket
)

func GlobalManager() *Manager {
	return internalmcp.GlobalMCPManager()
}

// Client is a single MCP server connection, for callers that need a direct,
// deterministic tool call (e.g. connector.ActionConnector.Act) rather than
// registering a server's tools into an agent's tool registry for the LLM to
// decide when to call. Sequence: NewClient, Start, Initialize, then
// CallTool/ListTools/etc.; Close when done - see MCPClientManager.Connect
// (internal/tools/system/mcp/tool_manager.go) for the same sequence used
// internally.
type Client = internalmcp.Client

// NewClient creates a new MCP client for the given server config. It does
// not connect - call Start then Initialize on the result before use.
func NewClient(config ServerConfig) (*Client, error) {
	return internalmcp.NewClient(config)
}

func AddServer(name string, config McpServerConfig, scope ConfigScope) error {
	return internalmcp.AddMcpServer(name, config, scope)
}

func ReconnectServer(ctx context.Context, manager *Manager, serverName string, cwd string) error {
	return internalmcp.ReconnectMcpServer(ctx, manager, serverName, cwd)
}

func ParseMcpConfigFromFile(filePath string) (McpJsonConfig, []ValidationError) {
	return internalmcp.ParseMcpConfigFromFile(filePath)
}
