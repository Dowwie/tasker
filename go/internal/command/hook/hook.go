package hook

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/dgordon/tasker/internal/command"
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

func init() {
	hookCmd.AddCommand(parseOutputCmd)
	hookCmd.AddCommand(getPromptCmd)
	command.RootCmd.AddCommand(hookCmd)
}
