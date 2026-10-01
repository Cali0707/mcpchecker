package llmjudge

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mcpchecker/mcpchecker/pkg/mcpclient"
	"github.com/mcpchecker/mcpchecker/pkg/mcpproxy"
)

// judgeServerManager implements mcpproxy.ServerManager
// this allows us to use this server with pkg/agent
type judgeServerManager struct {
	server    *judgeServer
	requestID string
	tmpDir    string
}

// judgeServerProxy implements mcpproxy.Server
// this allows us to use this server with pkg/agent
type judgeServerProxy struct {
	server    *judgeServer
	requestID string
}

var _ mcpproxy.ServerManager = &judgeServerManager{}

func (m *judgeServerManager) GetMcpServerFiles() ([]string, error) {
	if m.tmpDir != "" {
		return []string{fmt.Sprintf("%s/%s", m.tmpDir, mcpproxy.McpServerFileName)}, nil
	}

	cfg := &mcpclient.MCPConfig{
		MCPServers: map[string]*mcpclient.ServerConfig{
			"judge": m.server.GetConfig(m.requestID),
		},
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return nil, err
	}

	err = cfg.ToFile(fmt.Sprintf("%s/%s", tmpDir, mcpproxy.McpServerFileName))
	if err != nil {
		rmErr := os.Remove(tmpDir)
		if rmErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove temp dir '%s': %w", tmpDir, rmErr))
		}

		return nil, err
	}

	m.tmpDir = tmpDir

	return []string{fmt.Sprintf("%s/%s", tmpDir, mcpproxy.McpServerFileName)}, nil
}

func (m *judgeServerManager) GetMcpServers() []mcpproxy.Server {
	return []mcpproxy.Server{&judgeServerProxy{server: m.server, requestID: m.requestID}}
}

// noop for llm judge
func (m *judgeServerManager) Start(ctx context.Context) error {
	return nil
}

func (m *judgeServerManager) Close() error {
	if m.tmpDir != "" {
		return os.RemoveAll(m.tmpDir)
	}

	return nil
}

func (m *judgeServerManager) GetAllCallHistory() *mcpproxy.CallHistory {
	return &mcpproxy.CallHistory{}
}

func (m *judgeServerManager) GetCallHistoryForServer(serverName string) (mcpproxy.CallHistory, bool) {
	if serverName == "judge" {
		return mcpproxy.CallHistory{}, true
	}
	return mcpproxy.CallHistory{}, false
}

var _ mcpproxy.Server = &judgeServerProxy{}

// noop for llm judge
func (s *judgeServerProxy) Run(ctx context.Context) error {
	return nil
}

func (s *judgeServerProxy) GetConfig() (*mcpclient.ServerConfig, error) {
	return s.server.GetConfig(s.requestID), nil
}

// GetName returns the name of the MCP server
func (s *judgeServerProxy) GetName() string {
	return "judge"
}

// GetAllowedTools returns all the tools the user allowed
func (s *judgeServerProxy) GetAllowedTools(ctx context.Context) ([]*mcp.Tool, error) {
	return []*mcp.Tool{s.server.GetSubmitJudgementTool()}, nil
}

// GetInstructions returns the server instructions from InitializeResult
func (s *judgeServerProxy) GetInstructions() string {
	return ""
}

// Close closes the MCP proxy server, but not the underlying client connection
func (s *judgeServerProxy) Close() error {
	return nil
}

// GetCallHistory returns all the MCP calls made while the proxy server was running
func (s *judgeServerProxy) GetCallHistory() mcpproxy.CallHistory {
	return mcpproxy.CallHistory{}
}

// WaitReady blocks until the server has initialized and is ready to serve
func (s *judgeServerProxy) WaitReady(ctx context.Context) error {
	return nil
}
