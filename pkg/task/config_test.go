package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mcpchecker/mcpchecker/pkg/assertion"
	"github.com/mcpchecker/mcpchecker/pkg/steps"
	"github.com/mcpchecker/mcpchecker/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCasePath = "testdata"
)

// mustMarshalStep marshals a util.Step to json.RawMessage for test expectations
func mustMarshalStep(step *util.Step) json.RawMessage {
	raw, err := json.Marshal(step)
	if err != nil {
		panic(err)
	}
	return raw
}

func TestFromFile(t *testing.T) {
	basePath, err := os.Getwd()
	require.NoError(t, err)

	basePath = filepath.Join(basePath, testCasePath)

	tt := map[string]struct {
		file      string
		expected  *TaskConfig
		expectErr bool
	}{
		"task with limits": {
			file: "task-with-limits.yaml",
			expected: &TaskConfig{
				TypeMeta: util.TypeMeta{
					APIVersion: "mcpchecker/v1alpha2",
					Kind:       KindTask,
				},
				Metadata: TaskMetadata{
					Name:       "task with limits",
					Difficulty: DifficultyMedium,
				},
				Spec: &TaskSpec{
					Limits: &util.Limits{
						Timeout:        "15m",
						CleanupTimeout: "2m",
					},
					Setup: []*steps.StepConfig{{
						Config: map[string]json.RawMessage{
							"script": json.RawMessage(`{"inline":"echo setup"}`),
						},
					}},
					Verify: []*steps.StepConfig{{
						Config: map[string]json.RawMessage{
							"script": json.RawMessage(`{"inline":"echo verify"}`),
						},
					}},
					Cleanup: []*steps.StepConfig{{
						Config: map[string]json.RawMessage{
							"script": json.RawMessage(`{"inline":"echo cleanup"}`),
						},
					}},
					Prompt: &util.Step{
						Inline: "Do something",
					},
				},
				basePath: basePath,
			},
		},
		"create pod inline no verify": {
			file: "create-pod-inline-no-verify.yaml",
			expected: &TaskConfig{
				TypeMeta: util.TypeMeta{
					Kind: KindTask,
				},
				Metadata: TaskMetadata{
					Name:       "create pod inline",
					Difficulty: DifficultyEasy,
				},
				Spec: &TaskSpec{
					Setup: []*steps.StepConfig{{
						Config: map[string]json.RawMessage{
							"script": mustMarshalStep(&util.Step{
								Inline: `#!/usr/bin/env bash
kubectl delete namespace create-pod-test --ignore-not-found
kubectl create namespace create-pod-test`,
							}),
						},
					}},
					Cleanup: []*steps.StepConfig{{
						Config: map[string]json.RawMessage{
							"script": mustMarshalStep(&util.Step{
								Inline: `#!/usr/bin/env bash
kubectl delete pod web-server -n create-pod-test --ignore-not-found
kubectl delete namespace create-pod-test --ignore-not-found`,
							}),
						},
					}},
					Prompt: &util.Step{
						Inline: "Please create a nginx pod named web-server in the create-pod-test namespace",
					},
				},
				basePath: basePath,
			},
		},
		"create pod inline": {
			file: "create-pod-inline.yaml",
			expected: &TaskConfig{
				TypeMeta: util.TypeMeta{
					Kind: KindTask,
				},
				Metadata: TaskMetadata{
					Name:       "create pod inline",
					Difficulty: DifficultyEasy,
				},
				Spec: &TaskSpec{
					Setup: []*steps.StepConfig{{
						Config: map[string]json.RawMessage{
							"script": mustMarshalStep(&util.Step{
								Inline: `#!/usr/bin/env bash
kubectl delete namespace create-pod-test --ignore-not-found
kubectl create namespace create-pod-test`,
							}),
						},
					}},
					Verify: []*steps.StepConfig{{
						Config: map[string]json.RawMessage{
							"script": mustMarshalStep(&util.Step{
								Inline: `#!/usr/bin/env bash
if kubectl wait --for=condition=Ready pod/web-server -n create-pod-test --timeout=120s; then
    exit 0
else
    exit 1
fi`,
							}),
						},
					}},
					Cleanup: []*steps.StepConfig{{
						Config: map[string]json.RawMessage{
							"script": mustMarshalStep(&util.Step{
								Inline: `#!/usr/bin/env bash
kubectl delete pod web-server -n create-pod-test --ignore-not-found
kubectl delete namespace create-pod-test --ignore-not-found`,
							}),
						},
					}},
					Prompt: &util.Step{
						Inline: "Please create a nginx pod named web-server in the create-pod-test namespace",
					},
				},
				basePath: basePath,
			},
		},
	}

	for tn, tc := range tt {
		t.Run(tn, func(t *testing.T) {
			got, err := FromFile(fmt.Sprintf("%s/%s", basePath, tc.file))
			if tc.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestReadTaskAssertions(t *testing.T) {
	tests := []struct {
		name           string
		assertionsYAML string
		requirePresent bool
	}{
		{
			name: "assertions with requirePresent",
			assertionsYAML: `
    assertions:
      requirePresent: true
      toolsUsed:
        - server: kubernetes
          tool: pods_create`,
			requirePresent: true,
		},
		{
			name: "requirePresent defaults to false",
			assertionsYAML: `
    assertions:
      toolsUsed:
        - server: kubernetes
          tool: pods_create`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(`apiVersion: mcpchecker/v1alpha2
kind: Task
metadata:
  name: assertion task
spec:` + tt.assertionsYAML)

			got, err := Read(data, "")
			require.NoError(t, err)
			require.NotNil(t, got.Spec.Assertions)
			assert.Equal(t, tt.requirePresent, got.Spec.Assertions.RequirePresent)
			assert.Equal(t, []assertion.ToolAssertion{{Server: "kubernetes", Tool: "pods_create"}}, got.Spec.Assertions.ToolsUsed)
		})
	}
}
