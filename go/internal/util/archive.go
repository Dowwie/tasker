package util

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	ArchiveVersion     = "1.0"
	ArchiveTypePlanning = "planning"
)

// ArchiveError represents an error related to archive operations.
type ArchiveError struct {
	Path    string
	Op      string
	Message string
}

func (e *ArchiveError) Error() string {
	return fmt.Sprintf("archive %s failed for %s: %s", e.Op, e.Path, e.Message)
}

// ArchiveManifest contains metadata about an archived planning directory.
type ArchiveManifest struct {
	Version        string                 `json:"version"`
	ArchiveType    string                 `json:"archive_type"`
	ProjectName    string                 `json:"project_name"`
	ArchiveID      string                 `json:"archive_id"`
	ArchivedAt     string                 `json:"archived_at"`
	SourceDir      string                 `json:"source_dir"`
	PhaseAtArchive string                 `json:"phase_at_archive,omitempty"`
	Contents       map[string][]string    `json:"contents"`
	TaskSummary    *ArchiveTaskSummary    `json:"task_summary,omitempty"`
}

// ArchiveTaskSummary summarizes tasks at archive time.
type ArchiveTaskSummary struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

// ArchiveResult contains information about a completed archive operation.
type ArchiveResult struct {
	ArchivePath string
	ArchiveID   string
	ArchivedAt  string
	ItemsCount  int
}

func timestampID() string {
	return time.Now().UTC().Format("20060102_150405")
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// ArchivePlanning archives planning artifacts from a planning directory.
// It creates a timestamped copy of the planning directory contents
// under archiveRoot/projectName/planning/timestamp/.
func ArchivePlanning(planningDir, archiveRoot, projectName string) (*ArchiveResult, error) {
	absPlanningDir, err := filepath.Abs(planningDir)
	if err != nil {
		return nil, &ArchiveError{Path: planningDir, Op: "resolve", Message: err.Error()}
	}

	info, err := os.Stat(absPlanningDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ArchiveError{Path: absPlanningDir, Op: "stat", Message: "planning directory does not exist"}
		}
		return nil, &ArchiveError{Path: absPlanningDir, Op: "stat", Message: err.Error()}
	}
	if !info.IsDir() {
		return nil, &ArchiveError{Path: absPlanningDir, Op: "validate", Message: "path is not a directory"}
	}

	stateFile := filepath.Join(absPlanningDir, "state.json")
	stateData, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ArchiveError{Path: stateFile, Op: "read", Message: "state.json not found - nothing to archive"}
		}
		return nil, &ArchiveError{Path: stateFile, Op: "read", Message: err.Error()}
	}

	var stateJSON map[string]interface{}
	if err := json.Unmarshal(stateData, &stateJSON); err != nil {
		return nil, &ArchiveError{Path: stateFile, Op: "parse", Message: err.Error()}
	}

	currentPhase := ""
	if phase, ok := stateJSON["phase"].(map[string]interface{}); ok {
		if current, ok := phase["current"].(string); ok {
			currentPhase = current
		}
	}

	archiveID := timestampID()
	archivePath := filepath.Join(archiveRoot, projectName, ArchiveTypePlanning, archiveID)
	if err := os.MkdirAll(archivePath, 0755); err != nil {
		return nil, &ArchiveError{Path: archivePath, Op: "create", Message: err.Error()}
	}

	dirsToArchive := []string{"inputs", "artifacts", "tasks", "reports"}
	contents := make(map[string][]string)
	itemsCount := 0

	for _, dirName := range dirsToArchive {
		srcDir := filepath.Join(absPlanningDir, dirName)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(srcDir)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			continue
		}

		dstDir := filepath.Join(archivePath, dirName)
		if err := copyDir(srcDir, dstDir); err != nil {
			return nil, &ArchiveError{Path: dstDir, Op: "copy", Message: err.Error()}
		}

		var fileNames []string
		for _, entry := range entries {
			fileNames = append(fileNames, entry.Name())
		}
		contents[dirName] = fileNames
		itemsCount += len(fileNames)
	}

	dstStateFile := filepath.Join(archivePath, "state.json")
	if err := copyFile(stateFile, dstStateFile); err != nil {
		return nil, &ArchiveError{Path: dstStateFile, Op: "copy", Message: err.Error()}
	}
	itemsCount++

	taskSummary := extractTaskSummary(stateJSON)

	archivedAt := nowISO()
	manifest := ArchiveManifest{
		Version:        ArchiveVersion,
		ArchiveType:    ArchiveTypePlanning,
		ProjectName:    projectName,
		ArchiveID:      archiveID,
		ArchivedAt:     archivedAt,
		SourceDir:      absPlanningDir,
		PhaseAtArchive: currentPhase,
		Contents:       contents,
		TaskSummary:    taskSummary,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, &ArchiveError{Path: archivePath, Op: "marshal", Message: err.Error()}
	}

	manifestPath := filepath.Join(archivePath, "archive-manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, &ArchiveError{Path: manifestPath, Op: "write", Message: err.Error()}
	}

	return &ArchiveResult{
		ArchivePath: archivePath,
		ArchiveID:   archiveID,
		ArchivedAt:  archivedAt,
		ItemsCount:  itemsCount,
	}, nil
}

func extractTaskSummary(stateJSON map[string]interface{}) *ArchiveTaskSummary {
	tasks, ok := stateJSON["tasks"].(map[string]interface{})
	if !ok {
		return nil
	}

	byStatus := make(map[string]int)
	for _, task := range tasks {
		if taskMap, ok := task.(map[string]interface{}); ok {
			if status, ok := taskMap["status"].(string); ok {
				byStatus[status]++
			}
		}
	}

	return &ArchiveTaskSummary{
		Total:    len(tasks),
		ByStatus: byStatus,
	}
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

const ArchiveTypeExecution = "execution"

// ArchiveExecution archives execution artifacts from a planning directory.
func ArchiveExecution(planningDir, archiveRoot, projectName string) (*ArchiveResult, error) {
	absPlanningDir, err := filepath.Abs(planningDir)
	if err != nil {
		return nil, &ArchiveError{Path: planningDir, Op: "resolve", Message: err.Error()}
	}

	info, err := os.Stat(absPlanningDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ArchiveError{Path: absPlanningDir, Op: "stat", Message: "planning directory does not exist"}
		}
		return nil, &ArchiveError{Path: absPlanningDir, Op: "stat", Message: err.Error()}
	}
	if !info.IsDir() {
		return nil, &ArchiveError{Path: absPlanningDir, Op: "validate", Message: "path is not a directory"}
	}

	stateFile := filepath.Join(absPlanningDir, "state.json")
	stateData, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ArchiveError{Path: stateFile, Op: "read", Message: "state.json not found"}
		}
		return nil, &ArchiveError{Path: stateFile, Op: "read", Message: err.Error()}
	}

	var stateJSON map[string]interface{}
	if err := json.Unmarshal(stateData, &stateJSON); err != nil {
		return nil, &ArchiveError{Path: stateFile, Op: "parse", Message: err.Error()}
	}

	archiveID := timestampID()
	archivePath := filepath.Join(archiveRoot, projectName, ArchiveTypeExecution, archiveID)
	if err := os.MkdirAll(archivePath, 0755); err != nil {
		return nil, &ArchiveError{Path: archivePath, Op: "create", Message: err.Error()}
	}

	dirsToArchive := []string{"bundles", "logs", "reports"}
	contents := make(map[string][]string)
	itemsCount := 0

	for _, dirName := range dirsToArchive {
		srcDir := filepath.Join(absPlanningDir, dirName)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(srcDir)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			continue
		}

		dstDir := filepath.Join(archivePath, dirName)
		if err := copyDir(srcDir, dstDir); err != nil {
			return nil, &ArchiveError{Path: dstDir, Op: "copy", Message: err.Error()}
		}

		var fileNames []string
		for _, entry := range entries {
			fileNames = append(fileNames, entry.Name())
		}
		contents[dirName] = fileNames
		itemsCount += len(fileNames)
	}

	dstStateFile := filepath.Join(archivePath, "state.json")
	if err := copyFile(stateFile, dstStateFile); err != nil {
		return nil, &ArchiveError{Path: dstStateFile, Op: "copy", Message: err.Error()}
	}
	itemsCount++

	taskSummary := extractTaskSummary(stateJSON)

	archivedAt := nowISO()
	manifest := ArchiveManifest{
		Version:     ArchiveVersion,
		ArchiveType: ArchiveTypeExecution,
		ProjectName: projectName,
		ArchiveID:   archiveID,
		ArchivedAt:  archivedAt,
		SourceDir:   absPlanningDir,
		Contents:    contents,
		TaskSummary: taskSummary,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, &ArchiveError{Path: archivePath, Op: "marshal", Message: err.Error()}
	}

	manifestPath := filepath.Join(archivePath, "archive-manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, &ArchiveError{Path: manifestPath, Op: "write", Message: err.Error()}
	}

	return &ArchiveResult{
		ArchivePath: archivePath,
		ArchiveID:   archiveID,
		ArchivedAt:  archivedAt,
		ItemsCount:  itemsCount,
	}, nil
}

// ArchiveInfo contains information about a single archive.
type ArchiveInfo struct {
	ArchiveID   string `json:"archive_id"`
	ArchiveType string `json:"archive_type"`
	ProjectName string `json:"project_name"`
	ArchivedAt  string `json:"archived_at"`
	Path        string `json:"path"`
}

// ListArchives lists all archives in the archive root, optionally filtered by project.
func ListArchives(archiveRoot string, projectFilter string) ([]ArchiveInfo, error) {
	var archives []ArchiveInfo

	if _, err := os.Stat(archiveRoot); os.IsNotExist(err) {
		return archives, nil
	}

	projects, err := os.ReadDir(archiveRoot)
	if err != nil {
		return nil, &ArchiveError{Path: archiveRoot, Op: "read", Message: err.Error()}
	}

	for _, project := range projects {
		if !project.IsDir() {
			continue
		}
		projectName := project.Name()
		if projectFilter != "" && projectName != projectFilter {
			continue
		}

		projectPath := filepath.Join(archiveRoot, projectName)
		archiveTypes, err := os.ReadDir(projectPath)
		if err != nil {
			continue
		}

		for _, archiveType := range archiveTypes {
			if !archiveType.IsDir() {
				continue
			}
			typeName := archiveType.Name()
			typePath := filepath.Join(projectPath, typeName)

			timestamps, err := os.ReadDir(typePath)
			if err != nil {
				continue
			}

			for _, ts := range timestamps {
				if !ts.IsDir() {
					continue
				}
				archivePath := filepath.Join(typePath, ts.Name())
				manifestPath := filepath.Join(archivePath, "archive-manifest.json")

				data, err := os.ReadFile(manifestPath)
				if err != nil {
					continue
				}

				var manifest ArchiveManifest
				if err := json.Unmarshal(data, &manifest); err != nil {
					continue
				}

				archives = append(archives, ArchiveInfo{
					ArchiveID:   manifest.ArchiveID,
					ArchiveType: manifest.ArchiveType,
					ProjectName: manifest.ProjectName,
					ArchivedAt:  manifest.ArchivedAt,
					Path:        archivePath,
				})
			}
		}
	}

	return archives, nil
}

// RestorePlanning restores a planning archive to the planning directory.
func RestorePlanning(archiveRoot, archiveID, planningDir, projectName string) error {
	archives, err := ListArchives(archiveRoot, projectName)
	if err != nil {
		return err
	}

	var archivePath string
	for _, a := range archives {
		if a.ArchiveID == archiveID && a.ArchiveType == ArchiveTypePlanning {
			archivePath = a.Path
			break
		}
	}

	if archivePath == "" {
		return &ArchiveError{Path: archiveID, Op: "find", Message: "archive not found"}
	}

	absPlanningDir, err := filepath.Abs(planningDir)
	if err != nil {
		return &ArchiveError{Path: planningDir, Op: "resolve", Message: err.Error()}
	}

	if err := os.MkdirAll(absPlanningDir, 0755); err != nil {
		return &ArchiveError{Path: absPlanningDir, Op: "create", Message: err.Error()}
	}

	entries, err := os.ReadDir(archivePath)
	if err != nil {
		return &ArchiveError{Path: archivePath, Op: "read", Message: err.Error()}
	}

	for _, entry := range entries {
		if entry.Name() == "archive-manifest.json" {
			continue
		}

		srcPath := filepath.Join(archivePath, entry.Name())
		dstPath := filepath.Join(absPlanningDir, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return &ArchiveError{Path: dstPath, Op: "copy", Message: err.Error()}
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return &ArchiveError{Path: dstPath, Op: "copy", Message: err.Error()}
			}
		}
	}

	return nil
}
