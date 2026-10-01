package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mcpchecker/mcpchecker/pkg/eval"
	"github.com/mcpchecker/mcpchecker/pkg/tokens"
)

func TestSummaryCommand(t *testing.T) {
	results := sampleResults()
	filePath := createTestResultsFile(t, results)

	cmd := NewSummaryCmd()
	cmd.SetArgs([]string{filePath})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("summary command failed: %v", err)
	}
}

func TestSummaryCommandWithTaskFilter(t *testing.T) {
	results := sampleResults()
	filePath := createTestResultsFile(t, results)

	cmd := NewSummaryCmd()
	cmd.SetArgs([]string{filePath, "--task", "task-1"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("summary command with --task filter failed: %v", err)
	}
}

func TestSummaryCommandJSONOutput(t *testing.T) {
	results := sampleResults()
	filePath := createTestResultsFile(t, results)

	cmd := NewSummaryCmd()
	cmd.SetArgs([]string{filePath, "--output", "json"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("summary command with --output json failed: %v", err)
	}
}

func TestSummaryCommandGitHubOutput(t *testing.T) {
	results := sampleResults()
	filePath := createTestResultsFile(t, results)

	cmd := NewSummaryCmd()
	cmd.SetArgs([]string{filePath, "--github-output"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("summary command with --github-output failed: %v", err)
	}
}

func TestSummaryCommandEmptyResults(t *testing.T) {
	filePath := createTestResultsFile(t, []*eval.EvalResult{})

	cmd := NewSummaryCmd()
	cmd.SetArgs([]string{filePath})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("summary command with empty results failed: %v", err)
	}
}

func TestSummaryCommandFileNotFound(t *testing.T) {
	cmd := NewSummaryCmd()
	cmd.SetArgs([]string{"/nonexistent/path/results.json"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err == nil {
		t.Error("summary command should fail with nonexistent file")
	}
}

func TestBuildSummaryOutput(t *testing.T) {
	results := sampleResults()
	summary := buildSummaryOutput("test.json", results)

	if summary.TasksTotal != 3 {
		t.Errorf("TasksTotal = %d, want 3", summary.TasksTotal)
	}

	if summary.TasksPassed != 2 {
		t.Errorf("TasksPassed = %d, want 2", summary.TasksPassed)
	}

	if len(summary.Tasks) != 3 {
		t.Errorf("len(Tasks) = %d, want 3", len(summary.Tasks))
	}

	// Check first task
	if summary.Tasks[0].Name != "task-1" {
		t.Errorf("Tasks[0].Name = %s, want task-1", summary.Tasks[0].Name)
	}
	if !summary.Tasks[0].TaskPassed {
		t.Error("Tasks[0].TaskPassed should be true")
	}

	// Check failed task
	if summary.Tasks[2].TaskError == "" {
		t.Error("Tasks[2].TaskError should not be empty")
	}
}

func TestBuildSummaryOutputPresenceResults(t *testing.T) {
	result := &eval.EvalResult{
		TaskName:            "presence-results",
		TaskPassed:          true,
		AllAssertionsPassed: false,
		AssertionResults: &eval.CompositeAssertionResult{
			ToolsUsed:         &eval.SingleAssertionResult{Passed: true},
			SkippedAssertions: []eval.AssertionPresenceResult{{Type: "toolsUsed", Server: "kubernetes", Target: "pods_create", Reason: "tool not present"}},
			PresenceFailures:  []eval.AssertionPresenceResult{{Type: "toolsNotUsed", Server: "kubernetes", Reason: "server not present"}},
		},
	}
	summary := buildSummaryOutput("test.json", []*eval.EvalResult{result})

	if summary.AssertionsTotal != 2 || summary.AssertionsPassed != 1 || summary.AssertionsSkipped != 1 {
		t.Errorf("assertion counts = %d/%d with %d skipped, want 1/2 with 1 skipped", summary.AssertionsPassed, summary.AssertionsTotal, summary.AssertionsSkipped)
	}
	if len(summary.Tasks[0].SkippedAssertions) != 1 || len(summary.Tasks[0].PresenceFailures) != 1 {
		t.Fatalf("task presence data = %d skips, %d failures, want 1 each", len(summary.Tasks[0].SkippedAssertions), len(summary.Tasks[0].PresenceFailures))
	}
	if len(summary.Tasks[0].FailedAssertions) != 1 || !strings.Contains(summary.Tasks[0].FailedAssertions[0], "Presence failure:") {
		t.Errorf("FailedAssertions = %#v, want strict presence failure", summary.Tasks[0].FailedAssertions)
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("failed to marshal summary: %v", err)
	}
	var roundTrip SummaryOutput
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("failed to unmarshal summary: %v", err)
	}
	if len(roundTrip.Tasks[0].SkippedAssertions) != 1 || len(roundTrip.Tasks[0].PresenceFailures) != 1 {
		t.Errorf("round-trip task data = %#v, want structured skips and presence failures", roundTrip.Tasks[0])
	}
}

func TestOutputTextSummary(t *testing.T) {
	results := sampleResults()
	summary := buildSummaryOutput("test.json", results)

	// Just ensure it doesn't panic
	outputTextSummary(results, summary)
}

func TestOutputTextSummaryAllPassed(t *testing.T) {
	results := []*eval.EvalResult{
		{
			TaskName:            "task-1",
			TaskPassed:          true,
			AllAssertionsPassed: true,
			AssertionResults: &eval.CompositeAssertionResult{
				ToolsUsed: &eval.SingleAssertionResult{Passed: true},
			},
		},
	}
	summary := buildSummaryOutput("test.json", results)

	// Just ensure it doesn't panic
	outputTextSummary(results, summary)
}

func TestOutputTextSummaryAllFailed(t *testing.T) {
	results := []*eval.EvalResult{
		{
			TaskName:            "task-1",
			TaskPassed:          false,
			TaskError:           "something went wrong",
			AllAssertionsPassed: false,
			AssertionResults: &eval.CompositeAssertionResult{
				ToolsUsed: &eval.SingleAssertionResult{Passed: false, Reason: "Tool not called"},
			},
		},
	}
	summary := buildSummaryOutput("test.json", results)

	// Just ensure it doesn't panic
	outputTextSummary(results, summary)
}

func TestOutputTextSummaryAgentExecutionError(t *testing.T) {
	results := []*eval.EvalResult{
		{
			TaskName:            "task-1",
			TaskPassed:          false,
			AgentExecutionError: true,
			AllAssertionsPassed: false,
		},
	}
	summary := buildSummaryOutput("test.json", results)

	// Just ensure it doesn't panic
	outputTextSummary(results, summary)
}

func TestBuildSummaryOutputWithTokenUsage(t *testing.T) {
	results := []*eval.EvalResult{
		{
			TaskName:            "task-1",
			TaskPassed:          true,
			AllAssertionsPassed: true,
			TokenEstimate: &tokens.Estimate{
				TotalTokens: 1000,
				Actual: &tokens.Usage{
					InputTokens:  600,
					OutputTokens: 400,
				},
			},
			JudgeTokenUsage: &tokens.Usage{
				InputTokens:  200,
				OutputTokens: 100,
			},
		},
		{
			TaskName:            "task-2",
			TaskPassed:          true,
			AllAssertionsPassed: true,
			TokenEstimate: &tokens.Estimate{
				TotalTokens: 500,
				Actual: &tokens.Usage{
					InputTokens:  300,
					OutputTokens: 200,
				},
			},
			JudgeTokenUsage: &tokens.Usage{
				InputTokens:  150,
				OutputTokens: 50,
			},
		},
	}

	summary := buildSummaryOutput("test.json", results)

	if summary.TotalTokensEstimate != 1500 {
		t.Errorf("TotalTokensEstimate = %d, want 1500", summary.TotalTokensEstimate)
	}
	if summary.AgentTotalInputTokens != 900 {
		t.Errorf("AgentTotalInputTokens = %d, want 900", summary.AgentTotalInputTokens)
	}
	if summary.AgentTotalOutputTokens != 600 {
		t.Errorf("AgentTotalOutputTokens = %d, want 600", summary.AgentTotalOutputTokens)
	}
	if summary.JudgeTotalInputTokens != 350 {
		t.Errorf("JudgeTotalInputTokens = %d, want 350", summary.JudgeTotalInputTokens)
	}
	if summary.JudgeTotalOutputTokens != 150 {
		t.Errorf("JudgeTotalOutputTokens = %d, want 150", summary.JudgeTotalOutputTokens)
	}

	// Check per-task values
	if summary.Tasks[0].AgentInputTokens != 600 {
		t.Errorf("Tasks[0].AgentInputTokens = %d, want 600", summary.Tasks[0].AgentInputTokens)
	}
	if summary.Tasks[1].JudgeOutputTokens != 50 {
		t.Errorf("Tasks[1].JudgeOutputTokens = %d, want 50", summary.Tasks[1].JudgeOutputTokens)
	}
}

func TestOutputGitHubSummaryContent(t *testing.T) {
	results := []*eval.EvalResult{
		{
			TaskName:            "task-1",
			TaskPassed:          true,
			AllAssertionsPassed: true,
			AssertionResults: &eval.CompositeAssertionResult{
				ToolsUsed: &eval.SingleAssertionResult{Passed: true},
			},
			TokenEstimate: &tokens.Estimate{
				TotalTokens: 1000,
				Actual: &tokens.Usage{
					InputTokens:  600,
					OutputTokens: 400,
				},
			},
			JudgeTokenUsage: &tokens.Usage{
				InputTokens:  200,
				OutputTokens: 100,
			},
		},
	}

	summary := buildSummaryOutput("test.json", results)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputGitHubSummary(summary)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	expectedLines := []string{
		"results-file=test.json",
		"tasks-total=1",
		"tasks-passed=1",
		"task-pass-rate=1.0000",
		"assertions-total=1",
		"assertions-passed=1",
		"assertion-pass-rate=1.0000",
		"tokens-estimated=1000",
		"agent-input-tokens=600",
		"agent-output-tokens=400",
		"judge-input-tokens=200",
		"judge-output-tokens=100",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing expected line %q\nGot:\n%s", expected, output)
		}
	}
}
