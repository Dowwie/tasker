package bundle

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dgordon/tasker/internal/errors"
	"github.com/dgordon/tasker/internal/schema"
	"github.com/dgordon/tasker/internal/state"
)

const BundleVersion = "1.3"

type Behavior struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type FileMapping struct {
	Path      string   `json:"path"`
	Action    string   `json:"action"`
	Layer     string   `json:"layer,omitempty"`
	Purpose   string   `json:"purpose,omitempty"`
	Behaviors []string `json:"behaviors,omitempty"`
}

type Dependencies struct {
	Tasks    []string `json:"tasks,omitempty"`
	Files    []string `json:"files,omitempty"`
	External []string `json:"external,omitempty"`
}

type AcceptanceCriterion struct {
	Criterion    string `json:"criterion"`
	Verification string `json:"verification"`
}

type Constraints struct {
	Language  string   `json:"language,omitempty"`
	Framework string   `json:"framework,omitempty"`
	Patterns  []string `json:"patterns,omitempty"`
	Forbidden []string `json:"forbidden,omitempty"`
	Testing   string   `json:"testing,omitempty"`
	Raw       string   `json:"raw,omitempty"`
}

type SpecRef struct {
	Quote    string `json:"quote,omitempty"`
	Location string `json:"location,omitempty"`
}

type Context struct {
	Domain       string   `json:"domain,omitempty"`
	Capability   string   `json:"capability,omitempty"`
	CapabilityID string   `json:"capability_id,omitempty"`
	SpecRef      *SpecRef `json:"spec_ref,omitempty"`
	SteelThread  bool     `json:"steel_thread,omitempty"`
}

type TransitionGuard struct {
	Condition   string `json:"condition,omitempty"`
	InvariantID string `json:"invariant_id,omitempty"`
}

type TransitionDetail struct {
	ID            string            `json:"id"`
	FromState     string            `json:"from_state"`
	ToState       string            `json:"to_state"`
	Trigger       string            `json:"trigger"`
	Guards        []TransitionGuard `json:"guards,omitempty"`
	IsFailurePath bool              `json:"is_failure_path,omitempty"`
}

type StateMachine struct {
	TransitionsCovered []string           `json:"transitions_covered,omitempty"`
	GuardsEnforced     []string           `json:"guards_enforced,omitempty"`
	StatesReached      []string           `json:"states_reached,omitempty"`
	TransitionsDetail  []TransitionDetail `json:"transitions_detail,omitempty"`
}

type ArtifactChecksums struct {
	CapabilityMap  string `json:"capability_map,omitempty"`
	PhysicalMap    string `json:"physical_map,omitempty"`
	Constraints    string `json:"constraints,omitempty"`
	TaskDefinition string `json:"task_definition,omitempty"`
}

type Checksums struct {
	Artifacts       ArtifactChecksums `json:"artifacts,omitempty"`
	DependencyFiles map[string]string `json:"dependency_files,omitempty"`
}

type Bundle struct {
	Version            string                `json:"version"`
	BundleCreatedAt    string                `json:"bundle_created_at"`
	TaskID             string                `json:"task_id"`
	Name               string                `json:"name"`
	Phase              int                   `json:"phase"`
	TargetDir          string                `json:"target_dir"`
	Context            Context               `json:"context,omitempty"`
	Behaviors          []Behavior            `json:"behaviors"`
	Files              []FileMapping         `json:"files"`
	Dependencies       Dependencies          `json:"dependencies,omitempty"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria"`
	Constraints        Constraints           `json:"constraints,omitempty"`
	StateMachine       *StateMachine         `json:"state_machine,omitempty"`
	Checksums          Checksums             `json:"checksums,omitempty"`
}

type BundleInfo struct {
	TaskID    string `json:"task_id"`
	Name      string `json:"name"`
	Phase     int    `json:"phase"`
	CreatedAt string `json:"created_at"`
	FilePath  string `json:"file_path"`
}

type Generator struct {
	planningDir string
	bundlesDir  string
	tasksDir    string
	artifactsDir string
	inputsDir   string
}

func NewGenerator(planningDir string) *Generator {
	return &Generator{
		planningDir:  planningDir,
		bundlesDir:   filepath.Join(planningDir, "bundles"),
		tasksDir:     filepath.Join(planningDir, "tasks"),
		artifactsDir: filepath.Join(planningDir, "artifacts"),
		inputsDir:    filepath.Join(planningDir, "inputs"),
	}
}

func (g *Generator) GenerateBundle(taskID string) (*Bundle, error) {
	taskDef, err := g.loadTaskDefinition(taskID)
	if err != nil {
		return nil, err
	}

	capMap, err := g.loadCapabilityMap()
	if err != nil {
		return nil, err
	}

	physMap, err := g.loadPhysicalMap()
	if err != nil {
		return nil, err
	}

	st, err := g.loadState()
	if err != nil {
		return nil, err
	}

	constraints := g.loadConstraints()

	behaviors := g.expandBehaviors(taskDef, capMap)
	files := g.collectFiles(taskDef, physMap)
	depFiles := g.findDependencyFiles(st, taskDef)

	context := g.buildContext(taskDef, capMap)

	checksums := g.computeChecksums(taskID, st.TargetDir, depFiles)

	bundle := &Bundle{
		Version:         BundleVersion,
		BundleCreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		TaskID:          taskID,
		Name:            taskDef.Name,
		Phase:           taskDef.Phase,
		TargetDir:       st.TargetDir,
		Context:         context,
		Behaviors:       behaviors,
		Files:           files,
		Dependencies: Dependencies{
			Tasks:    taskDef.DependsOn,
			Files:    depFiles,
			External: taskDef.External,
		},
		AcceptanceCriteria: taskDef.AcceptanceCriteria,
		Constraints:        constraints,
		Checksums:          checksums,
	}

	if taskDef.StateMachine != nil {
		bundle.StateMachine = g.expandStateMachine(taskDef.StateMachine)
	}

	if err := g.saveBundle(bundle); err != nil {
		return nil, err
	}

	return bundle, nil
}

func (g *Generator) GenerateReadyBundles() ([]*Bundle, []error) {
	sm := state.NewStateManager(g.planningDir)
	st, err := sm.Load()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to load state: %w", err)}
	}

	ready := state.GetReadyTasks(st)
	if len(ready) == 0 {
		return nil, nil
	}

	var bundles []*Bundle
	var errs []error

	for _, task := range ready {
		bundle, err := g.GenerateBundle(task.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("task %s: %w", task.ID, err))
			continue
		}
		bundles = append(bundles, bundle)
	}

	return bundles, errs
}

func (g *Generator) ValidateBundle(taskID string) (*schema.ValidationResult, error) {
	bundlePath := filepath.Join(g.bundlesDir, fmt.Sprintf("%s-bundle.json", taskID))

	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return nil, errors.IONotExists(bundlePath)
	}

	validator := schema.Default()
	if validator == nil {
		return nil, errors.ConfigMissing("schema directory not configured")
	}

	return validator.ValidateFile(schema.SchemaExecutionBundle, bundlePath)
}

type IntegrityResult struct {
	Valid        bool     `json:"valid"`
	MissingFiles []string `json:"missing_files,omitempty"`
	ChangedFiles []string `json:"changed_files,omitempty"`
}

func (g *Generator) ValidateIntegrity(taskID string) (*IntegrityResult, error) {
	bundle, err := g.LoadBundle(taskID)
	if err != nil {
		return nil, err
	}

	result := &IntegrityResult{Valid: true}

	for _, depFile := range bundle.Dependencies.Files {
		fullPath := g.resolveDepPath(bundle.TargetDir, depFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			result.MissingFiles = append(result.MissingFiles, depFile)
			result.Valid = false
		}
	}

	if bundle.Checksums.DependencyFiles != nil {
		for depFile, expectedChecksum := range bundle.Checksums.DependencyFiles {
			if expectedChecksum == "" {
				continue
			}
			fullPath := g.resolveDepPath(bundle.TargetDir, depFile)
			currentChecksum := fileChecksum(fullPath)
			if currentChecksum != expectedChecksum {
				result.ChangedFiles = append(result.ChangedFiles, depFile)
				result.Valid = false
			}
		}
	}

	artifactPaths := map[string]string{
		"capability_map":  filepath.Join(g.artifactsDir, "capability-map.json"),
		"physical_map":    filepath.Join(g.artifactsDir, "physical-map.json"),
		"constraints":     filepath.Join(g.inputsDir, "constraints.md"),
		"task_definition": filepath.Join(g.tasksDir, fmt.Sprintf("%s.json", taskID)),
	}

	checksumMap := map[string]string{
		"capability_map":  bundle.Checksums.Artifacts.CapabilityMap,
		"physical_map":    bundle.Checksums.Artifacts.PhysicalMap,
		"constraints":     bundle.Checksums.Artifacts.Constraints,
		"task_definition": bundle.Checksums.Artifacts.TaskDefinition,
	}

	for name, path := range artifactPaths {
		expected := checksumMap[name]
		if expected == "" {
			continue
		}
		current := fileChecksum(path)
		if current != expected {
			result.ChangedFiles = append(result.ChangedFiles, fmt.Sprintf("artifact:%s", name))
			result.Valid = false
		}
	}

	return result, nil
}

func (g *Generator) ListBundles() ([]BundleInfo, error) {
	if _, err := os.Stat(g.bundlesDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(g.bundlesDir)
	if err != nil {
		return nil, errors.IOReadFailed(g.bundlesDir, err)
	}

	var bundles []BundleInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-bundle.json") {
			continue
		}

		taskID := strings.TrimSuffix(entry.Name(), "-bundle.json")
		bundlePath := filepath.Join(g.bundlesDir, entry.Name())

		bundle, err := g.LoadBundle(taskID)
		if err != nil {
			continue
		}

		bundles = append(bundles, BundleInfo{
			TaskID:    bundle.TaskID,
			Name:      bundle.Name,
			Phase:     bundle.Phase,
			CreatedAt: bundle.BundleCreatedAt,
			FilePath:  bundlePath,
		})
	}

	sort.Slice(bundles, func(i, j int) bool {
		return bundles[i].TaskID < bundles[j].TaskID
	})

	return bundles, nil
}

func (g *Generator) CleanBundles() (int, error) {
	if _, err := os.Stat(g.bundlesDir); os.IsNotExist(err) {
		return 0, nil
	}

	entries, err := os.ReadDir(g.bundlesDir)
	if err != nil {
		return 0, errors.IOReadFailed(g.bundlesDir, err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-bundle.json") {
			continue
		}

		bundlePath := filepath.Join(g.bundlesDir, entry.Name())
		if err := os.Remove(bundlePath); err != nil {
			return count, errors.IOWriteFailed(bundlePath, err)
		}
		count++
	}

	return count, nil
}

func (g *Generator) LoadBundle(taskID string) (*Bundle, error) {
	bundlePath := filepath.Join(g.bundlesDir, fmt.Sprintf("%s-bundle.json", taskID))

	data, err := os.ReadFile(bundlePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.IONotExists(bundlePath)
		}
		return nil, errors.IOReadFailed(bundlePath, err)
	}

	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, errors.Internal("failed to parse bundle JSON", err)
	}

	return &bundle, nil
}

type TaskDefinition struct {
	ID                 string                `json:"id"`
	Name               string                `json:"name"`
	Phase              int                   `json:"phase"`
	DependsOn          []string              `json:"depends_on,omitempty"`
	Blocks             []string              `json:"blocks,omitempty"`
	Behaviors          []string              `json:"behaviors,omitempty"`
	Files              []FileMapping         `json:"files,omitempty"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria,omitempty"`
	External           []string              `json:"external,omitempty"`
	Context            *TaskContext          `json:"context,omitempty"`
	StateMachine       *TaskStateMachine     `json:"state_machine,omitempty"`
}

type TaskContext struct {
	Domain       string   `json:"domain,omitempty"`
	Capability   string   `json:"capability,omitempty"`
	CapabilityID string   `json:"capability_id,omitempty"`
	SpecRef      *SpecRef `json:"spec_ref,omitempty"`
	SteelThread  bool     `json:"steel_thread,omitempty"`
}

type TaskStateMachine struct {
	TransitionsCovered []string `json:"transitions_covered,omitempty"`
	GuardsEnforced     []string `json:"guards_enforced,omitempty"`
	StatesReached      []string `json:"states_reached,omitempty"`
}

func (g *Generator) loadTaskDefinition(taskID string) (*TaskDefinition, error) {
	taskPath := filepath.Join(g.tasksDir, fmt.Sprintf("%s.json", taskID))

	data, err := os.ReadFile(taskPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.IONotExists(taskPath)
		}
		return nil, errors.IOReadFailed(taskPath, err)
	}

	var taskDef TaskDefinition
	if err := json.Unmarshal(data, &taskDef); err != nil {
		return nil, errors.Internal("failed to parse task definition", err)
	}

	taskDef.ID = taskID
	return &taskDef, nil
}

type CapabilityMap struct {
	Domains []Domain `json:"domains"`
}

type Domain struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Capabilities []Capability `json:"capabilities"`
}

type Capability struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	SpecRef   *SpecRef         `json:"spec_ref,omitempty"`
	Behaviors []CapBehavior    `json:"behaviors"`
}

type CapBehavior struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

func (g *Generator) loadCapabilityMap() (*CapabilityMap, error) {
	capPath := filepath.Join(g.artifactsDir, "capability-map.json")

	data, err := os.ReadFile(capPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.IONotExists(capPath)
		}
		return nil, errors.IOReadFailed(capPath, err)
	}

	var capMap CapabilityMap
	if err := json.Unmarshal(data, &capMap); err != nil {
		return nil, errors.Internal("failed to parse capability map", err)
	}

	return &capMap, nil
}

type PhysicalMap struct {
	FileMapping []PhysicalFileMapping `json:"file_mapping"`
}

type PhysicalFileMapping struct {
	BehaviorID string        `json:"behavior_id"`
	Files      []FileMapping `json:"files,omitempty"`
	Tests      []FileMapping `json:"tests,omitempty"`
}

func (g *Generator) loadPhysicalMap() (*PhysicalMap, error) {
	physPath := filepath.Join(g.artifactsDir, "physical-map.json")

	data, err := os.ReadFile(physPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.IONotExists(physPath)
		}
		return nil, errors.IOReadFailed(physPath, err)
	}

	var physMap PhysicalMap
	if err := json.Unmarshal(data, &physMap); err != nil {
		return nil, errors.Internal("failed to parse physical map", err)
	}

	return &physMap, nil
}

func (g *Generator) loadState() (*state.State, error) {
	sm := state.NewStateManager(g.planningDir)
	return sm.Load()
}

func (g *Generator) loadConstraints() Constraints {
	constraintsPath := filepath.Join(g.inputsDir, "constraints.md")

	data, err := os.ReadFile(constraintsPath)
	if err != nil {
		return Constraints{}
	}

	raw := string(data)
	lowerRaw := strings.ToLower(raw)

	constraints := Constraints{Raw: raw}

	if strings.Contains(lowerRaw, "python") {
		constraints.Language = "Python"
	} else if strings.Contains(lowerRaw, "typescript") {
		constraints.Language = "TypeScript"
	} else if strings.Contains(lowerRaw, "go") || strings.Contains(lowerRaw, "golang") {
		constraints.Language = "Go"
	} else if strings.Contains(lowerRaw, "rust") {
		constraints.Language = "Rust"
	}

	if strings.Contains(lowerRaw, "fastapi") {
		constraints.Framework = "FastAPI"
	} else if strings.Contains(lowerRaw, "django") {
		constraints.Framework = "Django"
	} else if strings.Contains(lowerRaw, "flask") {
		constraints.Framework = "Flask"
	} else if strings.Contains(lowerRaw, "gin") {
		constraints.Framework = "Gin"
	}

	if strings.Contains(lowerRaw, "pytest") {
		constraints.Testing = "pytest"
	} else if strings.Contains(lowerRaw, "go test") {
		constraints.Testing = "go test"
	}

	var patterns []string
	if strings.Contains(lowerRaw, "protocol") {
		patterns = append(patterns, "Use Protocol for interfaces")
	}
	if strings.Contains(lowerRaw, "dataclass") {
		patterns = append(patterns, "Use dataclass for data structures")
	}
	if strings.Contains(lowerRaw, "factory") {
		patterns = append(patterns, "Use factory functions for construction")
	}
	if strings.Contains(lowerRaw, "interface") {
		patterns = append(patterns, "Use interfaces for abstraction")
	}
	constraints.Patterns = patterns

	return constraints
}

func (g *Generator) expandBehaviors(taskDef *TaskDefinition, capMap *CapabilityMap) []Behavior {
	var behaviors []Behavior

	behaviorIndex := make(map[string]*CapBehavior)
	for _, domain := range capMap.Domains {
		for _, cap := range domain.Capabilities {
			for i := range cap.Behaviors {
				behaviorIndex[cap.Behaviors[i].ID] = &cap.Behaviors[i]
			}
		}
	}

	for _, behaviorID := range taskDef.Behaviors {
		if capBehavior, ok := behaviorIndex[behaviorID]; ok {
			behaviors = append(behaviors, Behavior{
				ID:          capBehavior.ID,
				Name:        capBehavior.Name,
				Type:        capBehavior.Type,
				Description: capBehavior.Description,
			})
		} else {
			behaviors = append(behaviors, Behavior{
				ID:          behaviorID,
				Name:        fmt.Sprintf("Unknown behavior %s", behaviorID),
				Type:        "process",
				Description: "",
			})
		}
	}

	return behaviors
}

func (g *Generator) collectFiles(taskDef *TaskDefinition, physMap *PhysicalMap) []FileMapping {
	seenPaths := make(map[string]bool)
	var files []FileMapping

	for _, file := range taskDef.Files {
		if file.Path != "" && !seenPaths[file.Path] {
			files = append(files, file)
			seenPaths[file.Path] = true
		}
	}

	for _, behaviorID := range taskDef.Behaviors {
		for _, mapping := range physMap.FileMapping {
			if mapping.BehaviorID != behaviorID {
				continue
			}

			for _, file := range mapping.Files {
				if file.Path != "" && !seenPaths[file.Path] {
					file.Behaviors = []string{behaviorID}
					files = append(files, file)
					seenPaths[file.Path] = true
				}
			}

			for _, test := range mapping.Tests {
				if test.Path != "" && !seenPaths[test.Path] {
					test.Layer = "test"
					test.Behaviors = []string{behaviorID}
					files = append(files, test)
					seenPaths[test.Path] = true
				}
			}
		}
	}

	return files
}

func (g *Generator) findDependencyFiles(st *state.State, taskDef *TaskDefinition) []string {
	var files []string

	for _, depID := range taskDef.DependsOn {
		depTask, exists := st.Tasks[depID]
		if !exists {
			continue
		}
		files = append(files, depTask.FilesCreated...)
	}

	return files
}

func (g *Generator) buildContext(taskDef *TaskDefinition, capMap *CapabilityMap) Context {
	ctx := Context{}

	if taskDef.Context != nil {
		ctx.Domain = taskDef.Context.Domain
		ctx.Capability = taskDef.Context.Capability
		ctx.CapabilityID = taskDef.Context.CapabilityID
		ctx.SpecRef = taskDef.Context.SpecRef
		ctx.SteelThread = taskDef.Context.SteelThread
	}

	if ctx.Domain == "" && len(taskDef.Behaviors) > 0 {
		for _, domain := range capMap.Domains {
			for _, cap := range domain.Capabilities {
				for _, behavior := range cap.Behaviors {
					if behavior.ID == taskDef.Behaviors[0] {
						ctx.Domain = domain.Name
						ctx.Capability = cap.Name
						ctx.CapabilityID = cap.ID
						ctx.SpecRef = cap.SpecRef
						return ctx
					}
				}
			}
		}
	}

	return ctx
}

func (g *Generator) expandStateMachine(taskFSM *TaskStateMachine) *StateMachine {
	if taskFSM == nil {
		return nil
	}

	return &StateMachine{
		TransitionsCovered: taskFSM.TransitionsCovered,
		GuardsEnforced:     taskFSM.GuardsEnforced,
		StatesReached:      taskFSM.StatesReached,
	}
}

func (g *Generator) computeChecksums(taskID, targetDir string, depFiles []string) Checksums {
	checksums := Checksums{
		Artifacts: ArtifactChecksums{
			CapabilityMap:  fileChecksum(filepath.Join(g.artifactsDir, "capability-map.json")),
			PhysicalMap:    fileChecksum(filepath.Join(g.artifactsDir, "physical-map.json")),
			Constraints:    fileChecksum(filepath.Join(g.inputsDir, "constraints.md")),
			TaskDefinition: fileChecksum(filepath.Join(g.tasksDir, fmt.Sprintf("%s.json", taskID))),
		},
		DependencyFiles: make(map[string]string),
	}

	for _, depFile := range depFiles {
		fullPath := g.resolveDepPath(targetDir, depFile)
		checksums.DependencyFiles[depFile] = fileChecksum(fullPath)
	}

	return checksums
}

func (g *Generator) resolveDepPath(targetDir, depFile string) string {
	targetName := filepath.Base(targetDir)
	parts := strings.Split(depFile, string(filepath.Separator))
	if len(parts) > 0 && parts[0] == targetName {
		depFile = filepath.Join(parts[1:]...)
	}
	return filepath.Join(targetDir, depFile)
}

func (g *Generator) saveBundle(bundle *Bundle) error {
	if err := os.MkdirAll(g.bundlesDir, 0755); err != nil {
		return errors.IOWriteFailed(g.bundlesDir, err)
	}

	bundlePath := filepath.Join(g.bundlesDir, fmt.Sprintf("%s-bundle.json", bundle.TaskID))

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return errors.Internal("failed to serialize bundle", err)
	}

	if err := os.WriteFile(bundlePath, data, 0644); err != nil {
		return errors.IOWriteFailed(bundlePath, err)
	}

	return nil
}

func fileChecksum(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)[:16]
}
