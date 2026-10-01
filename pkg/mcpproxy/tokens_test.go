package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mcpchecker/mcpchecker/pkg/mcpclient"
)

// testServer implements Server for testing
type testServer struct {
	name         string
	instructions string
	allowedTools []*mcp.Tool
	toolsErr     error
}

func (s *testServer) Run(_ context.Context) error                 { return nil }
func (s *testServer) GetConfig() (*mcpclient.ServerConfig, error) { return nil, nil }
func (s *testServer) GetName() string                             { return s.name }
func (s *testServer) GetAllowedTools(_ context.Context) ([]*mcp.Tool, error) {
	return s.allowedTools, s.toolsErr
}
func (s *testServer) GetInstructions() string           { return s.instructions }
func (s *testServer) Close() error                      { return nil }
func (s *testServer) GetCallHistory() CallHistory       { return CallHistory{} }
func (s *testServer) WaitReady(_ context.Context) error { return nil }

func TestComputeCallHistoryTokens_NilHistory(t *testing.T) {
	// Should not panic
	errStr := ComputeCallHistoryTokens(nil)
	assert.Empty(t, errStr)
}

func TestComputeCallHistoryTokens_EmptyHistory(t *testing.T) {
	history := &CallHistory{}
	errStr := ComputeCallHistoryTokens(history)
	assert.Empty(t, errStr)
}

func TestComputeCallHistoryTokens_ToolCalls(t *testing.T) {
	args := map[string]any{"query": "hello world"}
	argsRaw := json.RawMessage(`{"query":"hello world"}`)

	history := &CallHistory{
		ToolCalls: []*ToolCall{
			{
				CallRecord: CallRecord{ServerName: "srv1", Success: true},
				ToolName:   "search",
				Request: &mcp.CallToolRequest{
					Params: &mcp.CallToolParamsRaw{
						Name:      "search",
						Arguments: argsRaw,
					},
				},
				Result: &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: "found 3 results for hello world"},
					},
				},
			},
		},
	}

	errStr := ComputeCallHistoryTokens(history)
	assert.Empty(t, errStr)

	require.NotNil(t, history.ToolCalls[0].Tokens)
	assert.Greater(t, history.ToolCalls[0].Tokens.InputTokens, int64(0))
	assert.Greater(t, history.ToolCalls[0].Tokens.OutputTokens, int64(0))
	assert.Equal(t,
		history.ToolCalls[0].Tokens.InputTokens+history.ToolCalls[0].Tokens.OutputTokens,
		history.ToolCalls[0].Tokens.TotalTokens,
	)

	_ = args // suppress unused
}

func TestComputeCallHistoryTokens_NilRequestAndResult(t *testing.T) {
	history := &CallHistory{
		ToolCalls: []*ToolCall{
			{
				CallRecord: CallRecord{ServerName: "srv1", Success: false},
				ToolName:   "broken",
				Request:    nil,
				Result:     nil,
			},
		},
	}

	errStr := ComputeCallHistoryTokens(history)
	assert.Empty(t, errStr)

	require.NotNil(t, history.ToolCalls[0].Tokens)
	assert.Equal(t, int64(0), history.ToolCalls[0].Tokens.InputTokens)
	assert.Equal(t, int64(0), history.ToolCalls[0].Tokens.OutputTokens)
	assert.Equal(t, int64(0), history.ToolCalls[0].Tokens.TotalTokens)
}

func TestComputeCallHistoryTokens_ResourceReads(t *testing.T) {
	history := &CallHistory{
		ResourceReads: []*ResourceRead{
			{
				CallRecord: CallRecord{ServerName: "srv1", Success: true},
				URI:        "file:///path/to/resource.txt",
				Request: &mcp.ReadResourceRequest{
					Params: &mcp.ReadResourceParams{
						URI: "file:///path/to/resource.txt",
					},
				},
				Result: &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{
						{
							URI:  "file:///path/to/resource.txt",
							Text: "This is the content of the resource file with some meaningful text.",
						},
					},
				},
			},
		},
	}

	errStr := ComputeCallHistoryTokens(history)
	assert.Empty(t, errStr)

	require.NotNil(t, history.ResourceReads[0].Tokens)
	assert.Greater(t, history.ResourceReads[0].Tokens.InputTokens, int64(0))
	assert.Greater(t, history.ResourceReads[0].Tokens.OutputTokens, int64(0))
}

func TestComputeCallHistoryTokens_PromptGets(t *testing.T) {
	argsMap := map[string]string{"topic": "testing"}

	history := &CallHistory{
		PromptGets: []*PromptGet{
			{
				CallRecord: CallRecord{ServerName: "srv1", Success: true},
				Name:       "code-review",
				Request: &mcp.GetPromptRequest{
					Params: &mcp.GetPromptParams{
						Name:      "code-review",
						Arguments: argsMap,
					},
				},
				Result: &mcp.GetPromptResult{
					Messages: []*mcp.PromptMessage{
						{
							Role: "user",
							Content: &mcp.TextContent{
								Text: "Please review this code for best practices.",
							},
						},
					},
				},
			},
		},
	}

	errStr := ComputeCallHistoryTokens(history)
	assert.Empty(t, errStr)

	require.NotNil(t, history.PromptGets[0].Tokens)
	assert.Greater(t, history.PromptGets[0].Tokens.InputTokens, int64(0))
	assert.Greater(t, history.PromptGets[0].Tokens.OutputTokens, int64(0))
}

func TestComputeSchemaTokens_NoServers(t *testing.T) {
	total, err := ComputeSchemaTokens(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestComputeSchemaTokens_EmptyServer(t *testing.T) {
	servers := []Server{
		&testServer{name: "empty"},
	}
	total, err := ComputeSchemaTokens(context.Background(), servers)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestComputeSchemaTokens_WithToolDefinitions(t *testing.T) {
	servers := []Server{
		&testServer{
			name: "test-server",
			allowedTools: []*mcp.Tool{
				{
					Name:        "search",
					Description: "Search for documents matching a query",
					InputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query": map[string]any{
								"type":        "string",
								"description": "The search query",
							},
						},
						"required": []string{"query"},
					},
				},
				{
					Name:        "read_file",
					Description: "Read the contents of a file at the given path",
					InputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path": map[string]any{
								"type":        "string",
								"description": "Path to the file",
							},
						},
						"required": []string{"path"},
					},
				},
			},
		},
	}

	total, err := ComputeSchemaTokens(context.Background(), servers)
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))
}

func TestComputeSchemaTokens_WithInstructions(t *testing.T) {
	servers := []Server{
		&testServer{
			name:         "instructed-server",
			instructions: "You are a helpful assistant that searches documents.",
		},
	}

	total, err := ComputeSchemaTokens(context.Background(), servers)
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))
}

func TestComputeSchemaTokens_CombinesToolsAndInstructions(t *testing.T) {
	servers := []Server{
		&testServer{
			name:         "combined-server",
			instructions: "You are a helpful assistant.",
			allowedTools: []*mcp.Tool{
				{
					Name:        "search",
					Description: "Search for documents",
				},
			},
		},
	}

	total, err := ComputeSchemaTokens(context.Background(), servers)
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))

	// Should be more than just instructions alone
	instrOnly := []Server{
		&testServer{
			name:         "instr-only",
			instructions: "You are a helpful assistant.",
		},
	}
	instrTotal, _ := ComputeSchemaTokens(context.Background(), instrOnly)
	assert.Greater(t, total, instrTotal)
}

func TestComputeSchemaTokens_MultipleServers(t *testing.T) {
	singleServer := []Server{
		&testServer{
			name:         "srv1",
			instructions: "First server instructions.",
			allowedTools: []*mcp.Tool{
				{Name: "tool1", Description: "First tool"},
			},
		},
	}

	twoServers := []Server{
		&testServer{
			name:         "srv1",
			instructions: "First server instructions.",
			allowedTools: []*mcp.Tool{
				{Name: "tool1", Description: "First tool"},
			},
		},
		&testServer{
			name:         "srv2",
			instructions: "Second server instructions.",
			allowedTools: []*mcp.Tool{
				{Name: "tool2", Description: "Second tool"},
			},
		},
	}

	singleTotal, _ := ComputeSchemaTokens(context.Background(), singleServer)
	doubleTotal, _ := ComputeSchemaTokens(context.Background(), twoServers)

	assert.Greater(t, doubleTotal, singleTotal)
}

func TestComputeSchemaTokens_NilInputSchema(t *testing.T) {
	servers := []Server{
		&testServer{
			name: "srv",
			allowedTools: []*mcp.Tool{
				{
					Name:        "simple_tool",
					Description: "A tool with no input schema",
					InputSchema: nil,
				},
			},
		},
	}

	total, err := ComputeSchemaTokens(context.Background(), servers)
	require.NoError(t, err)
	assert.Greater(t, total, int64(0)) // should still count name + description
}

func TestComputeSchemaTokens_ReturnsToolDiscoveryError(t *testing.T) {
	wantErr := errors.New("list tools failed")
	servers := []Server{
		&testServer{name: "broken-server", toolsErr: wantErr},
	}

	_, err := ComputeSchemaTokens(context.Background(), servers)
	require.ErrorIs(t, err, wantErr)
	assert.Contains(t, err.Error(), "broken-server")
}
