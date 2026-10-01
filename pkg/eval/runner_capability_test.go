package eval

import (
	"context"
	"errors"
	"testing"

	"github.com/mcpchecker/mcpchecker/pkg/mcpproxy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capabilityTestServer struct {
	mcpproxy.Server
	name      string
	tools     []*mcp.Tool
	discovery error
	toolCalls int
}

func (s *capabilityTestServer) GetName() string { return s.name }

func (s *capabilityTestServer) GetAllowedTools(context.Context) ([]*mcp.Tool, error) {
	s.toolCalls++
	return s.tools, s.discovery
}

type capabilityTestManager struct {
	mcpproxy.ServerManager
	servers    []mcpproxy.Server
	history    *mcpproxy.CallHistory
	serverGets int
}

func (m *capabilityTestManager) GetMcpServers() []mcpproxy.Server {
	m.serverGets++
	return m.servers
}

func (m *capabilityTestManager) GetAllCallHistory() *mcpproxy.CallHistory {
	return m.history
}

func TestEvaluateTaskAssertionsResolvesLiveCapabilities(t *testing.T) {
	server := &capabilityTestServer{
		name: "available",
		tools: []*mcp.Tool{
			{Name: "present"},
		},
	}
	manager := &capabilityTestManager{
		servers: []mcpproxy.Server{server},
		history: &mcpproxy.CallHistory{},
	}

	tests := []struct {
		name          string
		assertions    []*TaskAssertions
		toolUsed      bool
		wantPassed    bool
		wantSkipped   int
		wantFailures  int
		wantToolsUsed *bool
	}{
		{
			name:          "present tool used passes",
			assertions:    []*TaskAssertions{{ToolsUsed: []ToolAssertion{{Server: "available", Tool: "present"}}}},
			toolUsed:      true,
			wantPassed:    true,
			wantToolsUsed: boolPointer(true),
		},
		{
			name:          "present tool unused fails",
			assertions:    []*TaskAssertions{{ToolsUsed: []ToolAssertion{{Server: "available", Tool: "present"}}}},
			wantPassed:    false,
			wantToolsUsed: boolPointer(false),
		},
		{
			name:        "absent tool defaults to skip",
			assertions:  []*TaskAssertions{{ToolsUsed: []ToolAssertion{{Server: "available", Tool: "missing"}}}},
			wantPassed:  true,
			wantSkipped: 1,
		},
		{
			name:        "absent server defaults to skip",
			assertions:  []*TaskAssertions{{ToolsUsed: []ToolAssertion{{Server: "missing-server", Tool: "missing"}}}},
			wantPassed:  true,
			wantSkipped: 1,
		},
		{
			name: "requirePresent fails for absent tool",
			assertions: []*TaskAssertions{{
				RequirePresent: true,
				ToolsUsed:      []ToolAssertion{{Server: "available", Tool: "missing"}},
			}},
			wantPassed:   false,
			wantFailures: 1,
		},
		{
			name: "eval-level assertion skips absent tool like task assertion",
			assertions: []*TaskAssertions{
				{ToolsUsed: []ToolAssertion{{Server: "available", Tool: "present"}}},
				{ToolsUsed: []ToolAssertion{{Server: "available", Tool: "missing"}}},
			},
			toolUsed:      true,
			wantPassed:    true,
			wantSkipped:   1,
			wantToolsUsed: boolPointer(true),
		},
		{
			name: "task and multiple eval assertion sets all contribute",
			assertions: []*TaskAssertions{
				{ToolsUsed: []ToolAssertion{{Server: "available", Tool: "present"}}},
				{ToolsUsed: []ToolAssertion{{Server: "available", Tool: "missing"}}},
				{ToolsNotUsed: []ToolAssertion{{Server: "available", Tool: "present"}}},
			},
			toolUsed:      true,
			wantPassed:    false,
			wantSkipped:   1,
			wantToolsUsed: boolPointer(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server.toolCalls = 0
			manager.serverGets = 0
			result := &EvalResult{}
			runner := &evalRunner{}
			manager.history = &mcpproxy.CallHistory{}
			if tt.toolUsed {
				manager.history.ToolCalls = []*mcpproxy.ToolCall{{
					CallRecord: mcpproxy.CallRecord{ServerName: "available"},
					ToolName:   "present",
				}}
			}

			err := runner.evaluateTaskAssertions(context.Background(), taskConfig{assertions: tt.assertions}, manager, result)

			require.NoError(t, err)
			assert.Equal(t, tt.wantPassed, result.AllAssertionsPassed)
			assert.Equal(t, 1, manager.serverGets, "the server list should be captured once per task evaluation")
			_, requiresAvailableServerTools := requiredToolDiscoveryServers(tt.assertions)[server.name]
			if requiresAvailableServerTools {
				assert.Equal(t, 1, server.toolCalls, "a referenced server should be queried once")
			} else {
				assert.Equal(t, 0, server.toolCalls, "an unrelated server should not be queried")
			}
			if tt.wantSkipped == 0 && tt.wantFailures == 0 && tt.wantToolsUsed == nil {
				require.NotNil(t, result.AssertionResults)
			} else {
				require.NotNil(t, result.AssertionResults)
				assert.Len(t, result.AssertionResults.SkippedAssertions, tt.wantSkipped)
				assert.Len(t, result.AssertionResults.PresenceFailures, tt.wantFailures)
				if tt.wantToolsUsed != nil {
					require.NotNil(t, result.AssertionResults.ToolsUsed)
					assert.Equal(t, *tt.wantToolsUsed, result.AssertionResults.ToolsUsed.Passed)
				}
			}
		})
	}
}

func TestEvaluateTaskAssertionsDiscoveryError(t *testing.T) {
	wantErr := errors.New("discovery unavailable")
	server := &capabilityTestServer{name: "broken-server", discovery: wantErr}
	manager := &capabilityTestManager{
		servers: []mcpproxy.Server{server},
		history: &mcpproxy.CallHistory{},
	}
	result := &EvalResult{}
	runner := &evalRunner{}

	err := runner.evaluateTaskAssertions(context.Background(), taskConfig{
		assertions: []*TaskAssertions{{ToolsUsed: []ToolAssertion{{Server: "broken-server", Tool: "anything"}}}},
	}, manager, result)

	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
	assert.Contains(t, err.Error(), "broken-server")
	assert.Nil(t, result.AssertionResults, "discovery failure must not be reported as a skipped assertion")
	assert.False(t, result.AllAssertionsPassed)
	assert.Equal(t, 1, server.toolCalls)
}

func TestEvaluateTaskAssertionsKeepsServersWithNoVisibleTools(t *testing.T) {
	server := &capabilityTestServer{name: "empty-server"}
	manager := &capabilityTestManager{
		servers: []mcpproxy.Server{server},
		history: &mcpproxy.CallHistory{},
	}
	result := &EvalResult{}
	runner := &evalRunner{}

	err := runner.evaluateTaskAssertions(context.Background(), taskConfig{
		assertions: []*TaskAssertions{{ResourcesRead: []ResourceAssertion{{Server: "empty-server", URI: "resource://expected"}}}},
	}, manager, result)

	require.NoError(t, err)
	require.NotNil(t, result.AssertionResults.ResourcesRead)
	assert.False(t, result.AssertionResults.ResourcesRead.Passed)
	assert.Empty(t, result.AssertionResults.SkippedAssertions)
	assert.Equal(t, 0, server.toolCalls)
}

func boolPointer(value bool) *bool { return &value }
