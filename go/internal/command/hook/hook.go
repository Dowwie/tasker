package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgordon/tasker/internal/command"
	"github.com/dgordon/tasker/internal/state"
	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Hook helper commands",
	Long:  `Utilities for Claude Code hooks to parse JSON without external dependencies.`,
}

var parseOutputCmd = &cobra.Command{
	Use:   "parse-output",
	Short: "Extract output from hook JSON",
	Long: `Reads hook JSON from stdin and extracts the output field.
Handles nested output structures from Claude Code PostToolUse hooks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)
		data, err := reader.ReadBytes('\x00')
		if err != nil && len(data) == 0 {
			data, _ = os.ReadFile("/dev/stdin")
		}

		if len(data) == 0 {
			return nil
		}

		var hookData map[string]interface{}
		if err := json.Unmarshal(data, &hookData); err != nil {
			return nil
		}

		output := extractOutput(hookData)
		if output != "" {
			fmt.Print(output)
		}
		return nil
	},
}

func extractOutput(data map[string]interface{}) string {
	if out, ok := data["output"]; ok {
		return extractStringOrNested(out)
	}
	if out, ok := data["tool_output"]; ok {
		return extractStringOrNested(out)
	}
	return ""
}

func extractStringOrNested(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case map[string]interface{}:
		if out, ok := val["output"]; ok {
			if s, ok := out.(string); ok {
				return s
			}
		}
	}
	return ""
}

var getPromptCmd = &cobra.Command{
	Use:   "get-prompt",
	Short: "Extract prompt from hook JSON",
	Long:  `Reads hook JSON from stdin and extracts the prompt field.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)
		data, err := reader.ReadBytes('\x00')
		if err != nil && len(data) == 0 {
			data, _ = os.ReadFile("/dev/stdin")
		}

		if len(data) == 0 {
			return nil
		}

		var hookData map[string]interface{}
		if err := json.Unmarshal(data, &hookData); err != nil {
			return nil
		}

		if prompt, ok := hookData["prompt"].(string); ok {
			fmt.Print(prompt)
		}
		return nil
	},
}

// subagentStopPayload represents the JSON payload from Claude Code SubagentStop hook
type subagentStopPayload struct {
	TranscriptPath string `json:"transcript_path"`
	SessionID      string `json:"session_id"`
}

// transcriptEntry represents a single line in the transcript JSONL
type transcriptEntry struct {
	Usage   *tokenUsage `json:"usage,omitempty"`
	Message *struct {
		Usage *tokenUsage `json:"usage,omitempty"`
	} `json:"message,omitempty"`
}

type tokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

var subagentStopCmd = &cobra.Command{
	Use:   "subagent-stop",
	Short: "Handle SubagentStop hook",
	Long: `Processes the SubagentStop hook from Claude Code.
Reads hook payload from stdin, parses the transcript to extract token usage,
and logs the usage to the tasker state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile("/dev/stdin")
		if err != nil || len(data) == 0 {
			return fmt.Errorf("failed to read payload from stdin")
		}

		var payload subagentStopPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("invalid JSON payload: %w", err)
		}

		if payload.TranscriptPath == "" {
			return fmt.Errorf("no transcript_path in payload")
		}

		inputTokens, outputTokens, err := parseTranscript(payload.TranscriptPath)
		if err != nil {
			return fmt.Errorf("failed to parse transcript: %w", err)
		}

		// Calculate cost (Sonnet rates: $3/M input, $15/M output)
		cost := (float64(inputTokens)/1_000_000)*3.0 + (float64(outputTokens)/1_000_000)*15.0

		// Truncate session ID to 8 chars like Python did
		sessionID := payload.SessionID
		if len(sessionID) > 8 {
			sessionID = sessionID[:8]
		}

		// Find planning directory and log tokens
		planningDir := findPlanningDir()
		if planningDir != "" {
			sm := state.NewStateManager(planningDir)
			if err := sm.LogTokens(sessionID, inputTokens, outputTokens, cost, ""); err != nil {
				fmt.Fprintf(os.Stderr, "failed to log tokens: %v\n", err)
			}
		}

		// Always print summary
		fmt.Printf("Session %s: %d tokens, $%.4f\n", sessionID, inputTokens+outputTokens, cost)
		return nil
	},
}

func parseTranscript(path string) (inputTokens, outputTokens int, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size for potentially large transcript lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		usage := entry.Usage
		if usage == nil && entry.Message != nil {
			usage = entry.Message.Usage
		}
		if usage != nil {
			inputTokens += usage.InputTokens
			outputTokens += usage.OutputTokens
		}
	}

	return inputTokens, outputTokens, scanner.Err()
}

func findPlanningDir() string {
	// Check TASKER_PLANNING_DIR env var first
	if dir := os.Getenv("TASKER_PLANNING_DIR"); dir != "" {
		return dir
	}

	// Walk up from cwd looking for project-planning directory
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "project-planning")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func init() {
	hookCmd.AddCommand(parseOutputCmd)
	hookCmd.AddCommand(getPromptCmd)
	hookCmd.AddCommand(subagentStopCmd)
	command.RootCmd.AddCommand(hookCmd)
}
