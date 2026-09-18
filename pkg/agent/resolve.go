package agent

import (
	"fmt"
	"strings"
)

const (
	builtinPrefix = "builtin."
)

func ResolveAgentRef(ref *AgentRef) (*AgentSpec, error) {
	if ref == nil {
		return nil, fmt.Errorf("agent ref must not be nil")
	}

	if ref.Type == "file" {
		if ref.Path == "" {
			return nil, fmt.Errorf("path must be specified when agent type is 'file'")
		}
		return LoadWithBuiltins(ref.Path)
	}

	if !strings.HasPrefix(ref.Type, builtinPrefix) {
		return nil, fmt.Errorf("agent type must be either 'file' or 'builtin.X' format, got: %q", ref.Type)
	}

	builtinType := strings.TrimPrefix(ref.Type, builtinPrefix)
	builtinAgent, ok := GetBuiltinType(builtinType)
	if !ok {
		return nil, fmt.Errorf("unknown builtin agent type: %q", builtinType)
	}

	if builtinAgent.RequiresModel() && ref.Model == "" {
		return nil, fmt.Errorf("builtin type %q requires a model to be specified", builtinType)
	}

	if err := builtinAgent.ValidateEnvironment(); err != nil {
		return nil, fmt.Errorf("builtin type %q environment validation failed: %w", builtinType, err)
	}

	agentSpec, err := builtinAgent.GetDefaults(ref.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get defaults for builtin agent %q: %w", builtinType, err)
	}
	if agentSpec.Builtin != nil {
		agentSpec.Builtin.UseResponsesAPI = ref.UseResponsesAPI
	}

	return agentSpec, nil
}
