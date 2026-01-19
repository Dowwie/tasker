package transform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/dgordon/tasker/internal/command"
	"github.com/spf13/cobra"
)

var (
	targetDir string

	pathSplitter  = regexp.MustCompile(`[/_.]`)
	sectionHeader = regexp.MustCompile(`^#{1,3}\s`)

	cachedSpec     string
	cachedCapMap   *CapabilityMap
	cachedState    *State
	sharedDataOnce sync.Once
	sharedDataErr  error

	taskCache   = make(map[string]*Task)
	taskCacheMu sync.RWMutex
)

type Task struct {
	Name         string           `json:"name"`
	Phase        int              `json:"phase"`
	Context      TaskContext      `json:"context"`
	Files        []FileInfo       `json:"files"`
	Behaviors    []string         `json:"behaviors"`
	Dependencies TaskDependencies `json:"dependencies"`
	Acceptance   []string         `json:"acceptance"`
}

type TaskContext struct {
	Domain       string `json:"domain"`
	DomainID     string `json:"domain_id"`
	Capability   string `json:"capability"`
	CapabilityID string `json:"capability_id"`
	SteelThread  bool   `json:"steel_thread"`
}

type FileInfo struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

type TaskDependencies struct {
	Tasks []string `json:"tasks"`
}

type State struct {
	Phase struct {
		Current string `json:"current"`
	} `json:"phase"`
	Tasks map[string]TaskState `json:"tasks"`
}

type TaskState struct {
	Status string   `json:"status"`
	Phase  int      `json:"phase"`
	Blocks []string `json:"blocks"`
}

type CapabilityMap struct {
	Domains []Domain `json:"domains"`
}

type Domain struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Capabilities []Capability `json:"capabilities"`
}

type Capability struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	SpecRef     string     `json:"spec_ref"`
	Behaviors   []Behavior `json:"behaviors"`
}

type Behavior struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type CapabilityContext struct {
	Domain     *DomainInfo     `json:"domain"`
	Capability *CapabilityInfo `json:"capability"`
	Behaviors  []BehaviorInfo  `json:"behaviors"`
}

type DomainInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CapabilityInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SpecRef     string `json:"spec_ref"`
}

type BehaviorInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type PreparedContext struct {
	TaskID               string            `json:"task_id"`
	Task                 *Task             `json:"task"`
	State                ContextState      `json:"state"`
	CapabilityContext    CapabilityContext `json:"capability_context"`
	RelevantSpecSections []string          `json:"relevant_spec_sections"`
	DependencyContext    []DependencyInfo  `json:"dependency_context"`
	SuggestedPriority    string            `json:"suggested_priority"`
	SuggestedLabels      []string          `json:"suggested_labels"`
}

type ContextState struct {
	Status string   `json:"status"`
	Phase  int      `json:"phase"`
	Blocks []string `json:"blocks"`
}

type DependencyInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	FilesCreated []string `json:"files_created"`
}

type BatchManifest struct {
	Issues []ManifestEntry `json:"issues"`
}

type ManifestEntry struct {
	TaskID       string   `json:"task_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Priority     string   `json:"priority"`
	Labels       []string `json:"labels"`
	Dependencies []string `json:"dependencies"`
}

type issueResult struct {
	taskID  string
	beadsID string
	deps    []string
	err     error
}

type depLinkResult struct {
	from    string
	to      string
	success bool
}

var transformCmd = &cobra.Command{
	Use:   "transform",
	Short: "Transform tasker tasks to beads issues",
	Long: `Commands for transforming Tasker task definitions into Beads issues.

This bridges the planning phase with persistent issue tracking by:
- Preparing task context for LLM enrichment
- Creating beads issues from enriched descriptions
- Batch creating issues with dependency linking`,
}

var contextCmd = &cobra.Command{
	Use:   "context [task-id | --all]",
	Short: "Prepare context for task enrichment",
	Long: `Extracts structural data from task files and saves context bundles.

Use --all to prepare context for all tasks, or specify a single task ID.
Context files are saved to project-planning/beads-export/`,
	RunE: runContext,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show transformation status",
	Long:  `Displays the current status of the transformation workflow including task counts and beads initialization state.`,
	RunE:  runStatus,
}

var initTargetCmd = &cobra.Command{
	Use:   "init-target <target-dir> [prefix]",
	Short: "Initialize beads in target directory",
	Long: `Initializes beads in the target directory for issue tracking.

Runs 'bd init <PREFIX>' followed by 'bd onboard' to set up the beads system.
Default prefix is 'TASK' if not specified.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runInitTarget,
}

var createCmd = &cobra.Command{
	Use:   "create <task-id> <enriched-file>",
	Short: "Create beads issue from enriched description",
	Long: `Creates a single beads issue from an enriched JSON description file.

The enriched file should contain: task_id, title, description, priority, labels.`,
	Args: cobra.ExactArgs(2),
	RunE: runCreate,
}

var batchCreateCmd = &cobra.Command{
	Use:   "batch-create <manifest-file>",
	Short: "Create multiple issues from manifest",
	Long: `Creates multiple beads issues from a manifest file in parallel.

Phase 1: Creates all issues concurrently
Phase 2: Links dependencies between issues

Outputs task-to-beads-mapping.json with the mapping of task IDs to beads IDs.`,
	Args: cobra.ExactArgs(1),
	RunE: runBatchCreate,
}

var contextAll bool

func init() {
	transformCmd.PersistentFlags().StringVarP(&targetDir, "target-dir", "t", "", "Target directory for beads management")

	contextCmd.Flags().BoolVar(&contextAll, "all", false, "Prepare context for all tasks")

	transformCmd.AddCommand(contextCmd)
	transformCmd.AddCommand(statusCmd)
	transformCmd.AddCommand(initTargetCmd)
	transformCmd.AddCommand(createCmd)
	transformCmd.AddCommand(batchCreateCmd)

	command.RootCmd.AddCommand(transformCmd)
}

func getPlanningDir() string {
	return command.GetPlanningDir()
}

func getTasksDir() string {
	return filepath.Join(getPlanningDir(), "tasks")
}

func getArtifactsDir() string {
	return filepath.Join(getPlanningDir(), "artifacts")
}

func getInputsDir() string {
	return filepath.Join(getPlanningDir(), "inputs")
}

func getBeadsExportDir() string {
	return filepath.Join(getPlanningDir(), "beads-export")
}

func getTargetDir() string {
	if targetDir != "" {
		if abs, err := filepath.Abs(targetDir); err == nil {
			return abs
		}
		return targetDir
	}
	cwd, _ := os.Getwd()
	return cwd
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isBeadsInitialized(dir string) bool {
	return isDir(filepath.Join(dir, ".beads"))
}

func loadJSON[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func loadSharedData() error {
	sharedDataOnce.Do(func() {
		var wg sync.WaitGroup
		wg.Add(3)

		go func() {
			defer wg.Done()
			data, _ := os.ReadFile(filepath.Join(getInputsDir(), "spec.md"))
			cachedSpec = string(data)
		}()

		go func() {
			defer wg.Done()
			cm, _ := loadJSON[CapabilityMap](filepath.Join(getArtifactsDir(), "capability-map.json"))
			if cm == nil {
				cachedCapMap = &CapabilityMap{}
			} else {
				cachedCapMap = cm
			}
		}()

		go func() {
			defer wg.Done()
			s, _ := loadJSON[State](filepath.Join(getPlanningDir(), "state.json"))
			if s == nil {
				cachedState = &State{Tasks: make(map[string]TaskState)}
			} else {
				if s.Tasks == nil {
					s.Tasks = make(map[string]TaskState)
				}
				cachedState = s
			}
		}()

		wg.Wait()
	})
	return sharedDataErr
}

func loadTask(taskID string) (*Task, error) {
	taskCacheMu.RLock()
	if task, ok := taskCache[taskID]; ok {
		taskCacheMu.RUnlock()
		return task, nil
	}
	taskCacheMu.RUnlock()

	task, err := loadJSON[Task](filepath.Join(getTasksDir(), taskID+".json"))
	if err != nil {
		return nil, err
	}

	if task != nil {
		taskCacheMu.Lock()
		taskCache[taskID] = task
		taskCacheMu.Unlock()
	}

	return task, nil
}

func preloadTasks(taskIDs []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())

	for _, id := range taskIDs {
		wg.Add(1)
		go func(taskID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			loadTask(taskID)
		}(id)
	}
	wg.Wait()
}

func getAllTaskIDs() ([]string, error) {
	entries, err := os.ReadDir(getTasksDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 5 && name[0] == 'T' && name[len(name)-5:] == ".json" {
			ids = append(ids, name[:len(name)-5])
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func findCapabilityContext(capMap *CapabilityMap, task *Task) CapabilityContext {
	result := CapabilityContext{}

	for i := range capMap.Domains {
		domain := &capMap.Domains[i]
		if domain.Name != task.Context.Domain && domain.ID != task.Context.DomainID {
			continue
		}

		result.Domain = &DomainInfo{
			Name:        domain.Name,
			Description: domain.Description,
		}

		for j := range domain.Capabilities {
			cap := &domain.Capabilities[j]
			if cap.Name != task.Context.Capability && cap.ID != task.Context.CapabilityID {
				continue
			}

			result.Capability = &CapabilityInfo{
				Name:        cap.Name,
				Description: cap.Description,
				SpecRef:     cap.SpecRef,
			}

			behaviorSet := make(map[string]struct{}, len(task.Behaviors))
			for _, b := range task.Behaviors {
				behaviorSet[b] = struct{}{}
			}

			result.Behaviors = make([]BehaviorInfo, 0, len(task.Behaviors))
			for k := range cap.Behaviors {
				behavior := &cap.Behaviors[k]
				if _, ok := behaviorSet[behavior.ID]; ok {
					result.Behaviors = append(result.Behaviors, BehaviorInfo{
						ID:          behavior.ID,
						Name:        behavior.Name,
						Description: behavior.Description,
						Type:        behavior.Type,
					})
				}
			}
			break
		}
		break
	}

	return result
}

var stopwords = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "with": {}, "from": {},
	"that": {}, "this": {}, "are": {}, "will": {}, "can": {}, "should": {},
}

func extractKeywords(task *Task, capCtx CapabilityContext) map[string]struct{} {
	keywords := make(map[string]struct{}, 32)

	addWords := func(s string) {
		for _, word := range strings.Fields(strings.ToLower(s)) {
			if _, stop := stopwords[word]; !stop && len(word) > 3 {
				keywords[word] = struct{}{}
			}
		}
	}

	addWords(task.Name)
	addWords(task.Context.Domain)
	addWords(task.Context.Capability)

	for i := range capCtx.Behaviors {
		addWords(capCtx.Behaviors[i].Name)
	}

	for i := range task.Files {
		parts := pathSplitter.Split(task.Files[i].Path, -1)
		for _, p := range parts {
			if len(p) > 3 {
				keywords[strings.ToLower(p)] = struct{}{}
			}
		}
	}

	return keywords
}

func splitSpecIntoSections(spec string) []string {
	lines := strings.Split(spec, "\n")
	var sections []string
	var current strings.Builder

	for _, line := range lines {
		if sectionHeader.MatchString(line) {
			if current.Len() > 0 {
				sections = append(sections, current.String())
				current.Reset()
			}
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}

	if current.Len() > 0 {
		sections = append(sections, current.String())
	}

	return sections
}

func extractRelevantSpecSections(spec string, keywords map[string]struct{}) []string {
	if spec == "" || len(keywords) == 0 {
		return nil
	}

	sections := splitSpecIntoSections(spec)
	type scored struct {
		section string
		score   int
	}
	candidates := make([]scored, 0, 8)

	for _, section := range sections {
		sectionLower := strings.ToLower(section)
		score := 0
		for kw := range keywords {
			if strings.Contains(sectionLower, kw) {
				score++
			}
		}

		if score >= 2 {
			s := strings.TrimSpace(section)
			if len(s) > 1500 {
				s = s[:1500] + "\n[...truncated...]"
			}
			candidates = append(candidates, scored{s, score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	result := make([]string, 0, 5)
	for i := 0; i < len(candidates) && i < 5; i++ {
		result = append(result, candidates[i].section)
	}
	return result
}

func getDependencyContext(task *Task) []DependencyInfo {
	deps := make([]DependencyInfo, 0, len(task.Dependencies.Tasks))

	for _, depID := range task.Dependencies.Tasks {
		depTask, err := loadTask(depID)
		if err != nil || depTask == nil {
			continue
		}

		files := make([]string, len(depTask.Files))
		for i, f := range depTask.Files {
			files[i] = f.Path
		}

		deps = append(deps, DependencyInfo{
			ID:           depID,
			Name:         depTask.Name,
			FilesCreated: files,
		})
	}

	return deps
}

var priorityMap = [...]string{"low", "critical", "high", "medium", "medium"}

func phaseToPriority(phase int) string {
	if phase < 0 || phase > 4 {
		return "low"
	}
	return priorityMap[phase]
}

func buildLabels(task *Task) []string {
	labels := make([]string, 0, 4)

	if task.Context.Domain != "" {
		labels = append(labels, "domain:"+strings.ToLower(strings.ReplaceAll(task.Context.Domain, " ", "-")))
	}
	if task.Context.Capability != "" {
		labels = append(labels, "capability:"+strings.ToLower(strings.ReplaceAll(task.Context.Capability, " ", "-")))
	}
	if task.Context.SteelThread {
		labels = append(labels, "steel-thread")
	}
	if task.Phase > 0 {
		labels = append(labels, fmt.Sprintf("phase:%d", task.Phase))
	}

	return labels
}

func prepareTaskContext(taskID string) (*PreparedContext, error) {
	task, err := loadTask(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, nil
	}

	if err := loadSharedData(); err != nil {
		return nil, err
	}

	capCtx := findCapabilityContext(cachedCapMap, task)
	keywords := extractKeywords(task, capCtx)
	relevantSpec := extractRelevantSpecSections(cachedSpec, keywords)
	depCtx := getDependencyContext(task)

	taskState := cachedState.Tasks[taskID]
	phase := taskState.Phase
	if phase == 0 {
		phase = task.Phase
	}

	return &PreparedContext{
		TaskID: taskID,
		Task:   task,
		State: ContextState{
			Status: taskState.Status,
			Phase:  phase,
			Blocks: taskState.Blocks,
		},
		CapabilityContext:    capCtx,
		RelevantSpecSections: relevantSpec,
		DependencyContext:    depCtx,
		SuggestedPriority:    phaseToPriority(task.Phase),
		SuggestedLabels:      buildLabels(task),
	}, nil
}

func saveContextForEnrichment(taskID string, ctx *PreparedContext) (string, error) {
	beadsExportDir := getBeadsExportDir()
	if err := os.MkdirAll(beadsExportDir, 0755); err != nil {
		return "", err
	}

	outputPath := filepath.Join(beadsExportDir, taskID+"-context.json")
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return "", err
	}

	return outputPath, os.WriteFile(outputPath, data, 0644)
}

func printContextSummary(ctx *PreparedContext) {
	fmt.Printf("Task: %s - %s\n", ctx.TaskID, ctx.Task.Name)
	fmt.Printf("  Phase: %d\n", ctx.State.Phase)
	fmt.Printf("  Priority: %s\n", ctx.SuggestedPriority)
	fmt.Printf("  Labels: %s\n", strings.Join(ctx.SuggestedLabels, ", "))
	fmt.Printf("  Spec sections found: %d\n", len(ctx.RelevantSpecSections))
	fmt.Printf("  Dependencies: %d\n", len(ctx.DependencyContext))
	fmt.Printf("  Behaviors: %d\n", len(ctx.CapabilityContext.Behaviors))
}

func runContext(cmd *cobra.Command, args []string) error {
	if contextAll {
		taskIDs, err := getAllTaskIDs()
		if err != nil {
			return err
		}

		preloadTasks(taskIDs)

		fmt.Printf("Prepared context for %d tasks\n\n", len(taskIDs))

		for _, taskID := range taskIDs {
			ctx, err := prepareTaskContext(taskID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", taskID, err)
				continue
			}
			if ctx == nil {
				continue
			}

			printContextSummary(ctx)
			if _, err := saveContextForEnrichment(taskID, ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save %s: %v\n", taskID, err)
			}
			fmt.Println()
		}

		fmt.Printf("\nContext files saved to: %s/\n", getBeadsExportDir())
		target := getTargetDir()
		if targetDir != "" {
			fmt.Printf("Target directory for beads: %s\n", target)
			fmt.Printf("  Beads initialized: %v\n", isBeadsInitialized(target))
		}
		return nil
	}

	if len(args) < 1 {
		return fmt.Errorf("usage: tasker transform context <task-id> | --all")
	}

	taskID := args[0]
	ctx, err := prepareTaskContext(taskID)
	if err != nil {
		return err
	}
	if ctx == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	printContextSummary(ctx)
	outputPath, err := saveContextForEnrichment(taskID, ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\nFull context saved to: %s\n", outputPath)

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	taskIDs, err := getAllTaskIDs()
	if err != nil {
		return err
	}

	if err := loadSharedData(); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	target := getTargetDir()

	fmt.Printf("Source Project: %s\n", cwd)
	fmt.Printf("Tasker Tasks: %d\n", len(taskIDs))
	fmt.Printf("State phase: %s\n", cachedState.Phase.Current)

	beadsExportDir := getBeadsExportDir()
	if isDir(beadsExportDir) {
		exported, _ := filepath.Glob(filepath.Join(beadsExportDir, "*-context.json"))
		enriched, _ := filepath.Glob(filepath.Join(beadsExportDir, "*-enriched.json"))
		fmt.Printf("\nBeads export directory: %s\n", beadsExportDir)
		fmt.Printf("  Context files: %d\n", len(exported))
		fmt.Printf("  Enriched files: %d\n", len(enriched))
	}

	fmt.Printf("\nTarget Directory: %s\n", target)
	if isBeadsInitialized(target) {
		fmt.Println("  Beads: initialized")
		issuesDir := filepath.Join(target, ".beads", "issues")
		if issues, err := filepath.Glob(filepath.Join(issuesDir, "*.md")); err == nil {
			fmt.Printf("  Issues: %d\n", len(issues))
		}
	} else {
		fmt.Println("  Beads: not initialized")
		fmt.Println("  Run 'tasker transform init-target <dir>' to initialize")
	}

	return nil
}

func initBeadsInTarget(target, prefix string) (bool, string) {
	if isBeadsInitialized(target) {
		return true, fmt.Sprintf("Beads already initialized in %s", target)
	}

	initCmd := exec.Command("bd", "init", prefix)
	initCmd.Dir = target
	initCmd.Env = append(os.Environ(), "PWD="+target)

	if output, err := initCmd.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("Failed to initialize beads: %s", string(output))
	}

	onboardCmd := exec.Command("bd", "onboard")
	onboardCmd.Dir = target
	onboardCmd.Env = append(os.Environ(), "PWD="+target)

	if output, err := onboardCmd.CombinedOutput(); err != nil {
		return false, fmt.Sprintf("Beads initialized but onboarding failed: %s", string(output))
	}

	return true, fmt.Sprintf("Beads initialized and onboarded in %s with prefix '%s'", target, prefix)
}

func runInitTarget(cmd *cobra.Command, args []string) error {
	target, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	if !isDir(target) {
		return fmt.Errorf("target directory does not exist: %s", target)
	}

	prefix := "TASK"
	if len(args) > 1 {
		prefix = args[1]
	}

	success, msg := initBeadsInTarget(target, prefix)
	fmt.Println(msg)
	if !success {
		return fmt.Errorf("initialization failed")
	}
	return nil
}

var bdPriorityMap = map[string]string{
	"critical": "0",
	"high":     "1",
	"medium":   "2",
	"low":      "3",
}

func priorityToBdPriority(priority string) string {
	if p, ok := bdPriorityMap[strings.ToLower(priority)]; ok {
		return p
	}
	return "2"
}

func createBeadsIssue(taskID, title, description, priority string, labels []string) (bool, string) {
	target := getTargetDir()

	allLabels := make([]string, len(labels), len(labels)+1)
	copy(allLabels, labels)
	allLabels = append(allLabels, "tasker:"+taskID)

	cmdArgs := []string{
		"create", title,
		"-t", "task",
		"-p", priorityToBdPriority(priority),
		"--silent",
		"-l", strings.Join(allLabels, ","),
	}

	if description != "" {
		cmdArgs = append(cmdArgs, "-d", description)
	}

	bdCmd := exec.Command("bd", cmdArgs...)
	bdCmd.Dir = target
	bdCmd.Env = append(os.Environ(), "PWD="+target)

	output, err := bdCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return false, fmt.Sprintf("Failed to create issue: %s", string(exitErr.Stderr))
		}
		return false, fmt.Sprintf("Failed to create issue: %v", err)
	}

	issueID := strings.TrimSpace(string(output))
	if issueID == "" {
		return false, "No issue ID returned from bd create"
	}

	return true, issueID
}

func createIssuesParallel(entries []ManifestEntry, workers int) (map[string]string, map[string][]string, int, int) {
	target := getTargetDir()

	if !isBeadsInitialized(target) {
		success, msg := initBeadsInTarget(target, "TASK")
		if !success {
			fmt.Printf("  %s\n", msg)
			return nil, nil, 0, len(entries)
		}
		fmt.Printf("  %s\n", msg)
	}

	jobs := make(chan ManifestEntry, len(entries))
	results := make(chan issueResult, len(entries))

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				success, result := createBeadsIssue(
					entry.TaskID,
					entry.Title,
					entry.Description,
					entry.Priority,
					entry.Labels,
				)

				ir := issueResult{taskID: entry.TaskID}
				if success {
					ir.beadsID = result
					task, _ := loadTask(entry.TaskID)
					if task != nil && len(task.Dependencies.Tasks) > 0 {
						ir.deps = task.Dependencies.Tasks
					}
				} else {
					ir.err = fmt.Errorf("%s", result)
				}
				results <- ir
			}
		}()
	}

	for _, entry := range entries {
		jobs <- entry
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	taskToBeads := make(map[string]string, len(entries))
	taskDeps := make(map[string][]string)
	created, failed := 0, 0

	for r := range results {
		if r.err != nil {
			fmt.Printf("  Failed %s: %v\n", r.taskID, r.err)
			failed++
		} else {
			fmt.Printf("  Created: %s -> %s\n", r.taskID, r.beadsID)
			taskToBeads[r.taskID] = r.beadsID
			if len(r.deps) > 0 {
				taskDeps[r.taskID] = r.deps
			}
			created++
		}
	}

	return taskToBeads, taskDeps, created, failed
}

func linkDependenciesParallel(taskToBeads map[string]string, taskDeps map[string][]string, workers int) (int, int) {
	target := getTargetDir()
	env := append(os.Environ(), "PWD="+target)

	type depJob struct {
		taskID     string
		beadsID    string
		depTaskID  string
		depBeadsID string
	}

	var jobs []depJob
	for taskID, depTaskIDs := range taskDeps {
		beadsID, ok := taskToBeads[taskID]
		if !ok {
			continue
		}
		for _, depTaskID := range depTaskIDs {
			depBeadsID, ok := taskToBeads[depTaskID]
			if !ok {
				fmt.Printf("  Warning: Dependency %s not found for %s\n", depTaskID, taskID)
				continue
			}
			jobs = append(jobs, depJob{taskID, beadsID, depTaskID, depBeadsID})
		}
	}

	if len(jobs) == 0 {
		return 0, 0
	}

	jobCh := make(chan depJob, len(jobs))
	results := make(chan depLinkResult, len(jobs))

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				bdCmd := exec.Command("bd", "dep", "add", job.beadsID, job.depBeadsID, "-t", "blocks")
				bdCmd.Dir = target
				bdCmd.Env = env

				_, err := bdCmd.CombinedOutput()
				results <- depLinkResult{
					from:    job.taskID,
					to:      job.depTaskID,
					success: err == nil,
				}
			}
		}()
	}

	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)

	go func() {
		wg.Wait()
		close(results)
	}()

	successCount, failCount := 0, 0
	for r := range results {
		if r.success {
			successCount++
		} else {
			fmt.Printf("  Failed to link %s -> %s\n", r.from, r.to)
			failCount++
		}
	}

	return successCount, failCount
}

func runCreate(cmd *cobra.Command, args []string) error {
	taskID := args[0]
	descFile := args[1]

	data, err := os.ReadFile(descFile)
	if err != nil {
		return fmt.Errorf("description file not found: %s", descFile)
	}

	var enriched ManifestEntry
	if err := json.Unmarshal(data, &enriched); err != nil {
		return fmt.Errorf("failed to parse enriched file: %v", err)
	}

	target := getTargetDir()
	fmt.Printf("Creating issue in: %s\n", target)

	success, result := createBeadsIssue(
		taskID,
		enriched.Title,
		enriched.Description,
		enriched.Priority,
		enriched.Labels,
	)

	if success {
		fmt.Printf("Created beads issue: %s\n", result)
		return nil
	}
	return fmt.Errorf("%s", result)
}

func runBatchCreate(cmd *cobra.Command, args []string) error {
	manifestPath := args[0]
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("manifest not found: %s", manifestPath)
	}

	var manifest BatchManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %v", err)
	}

	target := getTargetDir()
	fmt.Printf("Creating issues in: %s\n", target)

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}

	taskIDs := make([]string, len(manifest.Issues))
	for i, entry := range manifest.Issues {
		taskIDs[i] = entry.TaskID
	}
	preloadTasks(taskIDs)

	fmt.Println("\n--- Phase 1: Creating issues ---")
	taskToBeads, taskDeps, created, failed := createIssuesParallel(manifest.Issues, workers)

	fmt.Printf("\nPhase 1 complete: %d created, %d failed\n", created, failed)

	if len(taskDeps) > 0 {
		fmt.Println("\n--- Phase 2: Linking dependencies ---")
		depSuccess, depFailed := linkDependenciesParallel(taskToBeads, taskDeps, workers)
		fmt.Printf("\nPhase 2 complete: %d links created, %d failed\n", depSuccess, depFailed)
	} else {
		fmt.Println("\nNo dependencies to link.")
	}

	beadsExportDir := getBeadsExportDir()
	mappingFile := filepath.Join(beadsExportDir, "task-to-beads-mapping.json")
	mappingData, _ := json.MarshalIndent(taskToBeads, "", "  ")
	if err := os.WriteFile(mappingFile, mappingData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save mapping: %v\n", err)
	} else {
		fmt.Printf("\nMapping saved to: %s\n", mappingFile)
	}

	return nil
}
