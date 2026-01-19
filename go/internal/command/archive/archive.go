package archive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgordon/tasker/internal/command"
	"github.com/dgordon/tasker/internal/util"
	"github.com/spf13/cobra"
)

var (
	archiveRoot   string
	projectFilter string
)

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive management commands",
	Long:  `Commands for archiving and restoring planning and execution artifacts.`,
}

var planningCmd = &cobra.Command{
	Use:   "planning <project-name>",
	Short: "Archive planning artifacts",
	Long:  `Archives planning artifacts (inputs, artifacts, tasks, reports, state.json) to a timestamped directory.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		planningDir := os.Getenv("TASKER_PLANNING_DIR")
		if planningDir == "" {
			planningDir = "project-planning"
		}

		if archiveRoot == "" {
			archiveRoot = filepath.Join(filepath.Dir(planningDir), "archive")
		}

		result, err := util.ArchivePlanning(planningDir, archiveRoot, projectName)
		if err != nil {
			return fmt.Errorf("archive failed: %w", err)
		}

		fmt.Printf("Archived planning artifacts to: %s\n", result.ArchivePath)
		fmt.Printf("Archive ID: %s\n", result.ArchiveID)
		fmt.Printf("Items archived: %d\n", result.ItemsCount)
		return nil
	},
}

var executionCmd = &cobra.Command{
	Use:   "execution <project-name>",
	Short: "Archive execution artifacts",
	Long:  `Archives execution artifacts (bundles, logs, reports, state.json) to a timestamped directory.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		planningDir := os.Getenv("TASKER_PLANNING_DIR")
		if planningDir == "" {
			planningDir = "project-planning"
		}

		if archiveRoot == "" {
			archiveRoot = filepath.Join(filepath.Dir(planningDir), "archive")
		}

		result, err := util.ArchiveExecution(planningDir, archiveRoot, projectName)
		if err != nil {
			return fmt.Errorf("archive failed: %w", err)
		}

		fmt.Printf("Archived execution artifacts to: %s\n", result.ArchivePath)
		fmt.Printf("Archive ID: %s\n", result.ArchiveID)
		fmt.Printf("Items archived: %d\n", result.ItemsCount)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List archives",
	Long:  `Lists all archives, optionally filtered by project name.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := os.Getenv("TASKER_PLANNING_DIR")
		if planningDir == "" {
			planningDir = "project-planning"
		}

		if archiveRoot == "" {
			archiveRoot = filepath.Join(filepath.Dir(planningDir), "archive")
		}

		archives, err := util.ListArchives(archiveRoot, projectFilter)
		if err != nil {
			return fmt.Errorf("list failed: %w", err)
		}

		if len(archives) == 0 {
			fmt.Println("No archives found")
			return nil
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			data, err := json.MarshalIndent(archives, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		} else {
			fmt.Printf("%-20s %-12s %-20s %s\n", "ARCHIVE ID", "TYPE", "PROJECT", "ARCHIVED AT")
			fmt.Println("-------------------- ------------ -------------------- -------------------------")
			for _, a := range archives {
				fmt.Printf("%-20s %-12s %-20s %s\n", a.ArchiveID, a.ArchiveType, a.ProjectName, a.ArchivedAt)
			}
		}
		return nil
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore <archive-id>",
	Short: "Restore from archive",
	Long:  `Restores a planning archive to the planning directory.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		archiveID := args[0]
		planningDir := os.Getenv("TASKER_PLANNING_DIR")
		if planningDir == "" {
			planningDir = "project-planning"
		}

		if archiveRoot == "" {
			archiveRoot = filepath.Join(filepath.Dir(planningDir), "archive")
		}

		if err := util.RestorePlanning(archiveRoot, archiveID, planningDir, projectFilter); err != nil {
			return fmt.Errorf("restore failed: %w", err)
		}

		fmt.Printf("Restored archive %s to %s\n", archiveID, planningDir)
		return nil
	},
}

func init() {
	archiveCmd.PersistentFlags().StringVar(&archiveRoot, "archive-root", "", "Root directory for archives (default: ../archive)")

	listCmd.Flags().StringVar(&projectFilter, "project", "", "Filter by project name")
	listCmd.Flags().String("format", "text", "Output format: text or json")

	restoreCmd.Flags().StringVar(&projectFilter, "project", "", "Project name to search within")

	archiveCmd.AddCommand(planningCmd)
	archiveCmd.AddCommand(executionCmd)
	archiveCmd.AddCommand(listCmd)
	archiveCmd.AddCommand(restoreCmd)
	command.RootCmd.AddCommand(archiveCmd)
}
