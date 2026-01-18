package validate

import (
	"fmt"
	"sort"
)

type Task struct {
	ID          string
	DependsOn   []string
	Blocks      []string
	SteelThread bool
}

type CycleError struct {
	Cycle []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected: %v", e.Cycle)
}

type MissingDependencyError struct {
	TaskID      string
	MissingDeps []string
}

func (e *MissingDependencyError) Error() string {
	return fmt.Sprintf("task %s references non-existent dependencies: %v", e.TaskID, e.MissingDeps)
}

type SteelThreadError struct {
	Message string
}

func (e *SteelThreadError) Error() string {
	return e.Message
}

type ValidationResult struct {
	Valid    bool
	Errors   []error
	Warnings []string
}

func DetectCycles(tasks map[string]Task) *CycleError {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := make([]string, 0)

	var dfs func(taskID string) *CycleError
	dfs = func(taskID string) *CycleError {
		visited[taskID] = true
		recStack[taskID] = true
		path = append(path, taskID)

		task, exists := tasks[taskID]
		if !exists {
			path = path[:len(path)-1]
			recStack[taskID] = false
			return nil
		}

		for _, dep := range task.DependsOn {
			if !visited[dep] {
				if err := dfs(dep); err != nil {
					return err
				}
			} else if recStack[dep] {
				cycleStart := -1
				for i, id := range path {
					if id == dep {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := append(path[cycleStart:], dep)
					return &CycleError{Cycle: cycle}
				}
			}
		}

		path = path[:len(path)-1]
		recStack[taskID] = false
		return nil
	}

	taskIDs := make([]string, 0, len(tasks))
	for id := range tasks {
		taskIDs = append(taskIDs, id)
	}
	sort.Strings(taskIDs)

	for _, taskID := range taskIDs {
		if !visited[taskID] {
			if err := dfs(taskID); err != nil {
				return err
			}
		}
	}

	return nil
}

func CheckDependencyExistence(tasks map[string]Task) []MissingDependencyError {
	var errors []MissingDependencyError

	taskIDs := make([]string, 0, len(tasks))
	for id := range tasks {
		taskIDs = append(taskIDs, id)
	}
	sort.Strings(taskIDs)

	for _, taskID := range taskIDs {
		task := tasks[taskID]
		var missingDeps []string

		for _, dep := range task.DependsOn {
			if _, exists := tasks[dep]; !exists {
				missingDeps = append(missingDeps, dep)
			}
		}

		if len(missingDeps) > 0 {
			errors = append(errors, MissingDependencyError{
				TaskID:      taskID,
				MissingDeps: missingDeps,
			})
		}
	}

	return errors
}

func ValidateSteelThread(tasks map[string]Task) *SteelThreadError {
	var steelThreadTasks []Task
	for _, task := range tasks {
		if task.SteelThread {
			steelThreadTasks = append(steelThreadTasks, task)
		}
	}

	if len(steelThreadTasks) == 0 {
		return nil
	}

	steelThreadIDs := make(map[string]bool)
	for _, task := range steelThreadTasks {
		steelThreadIDs[task.ID] = true
	}

	for _, task := range steelThreadTasks {
		for _, dep := range task.DependsOn {
			depTask, exists := tasks[dep]
			if !exists {
				return &SteelThreadError{
					Message: fmt.Sprintf("steel thread task %s depends on non-existent task %s", task.ID, dep),
				}
			}
			if !depTask.SteelThread {
				return &SteelThreadError{
					Message: fmt.Sprintf("steel thread task %s depends on non-steel-thread task %s", task.ID, dep),
				}
			}
		}
	}

	if err := DetectCycles(tasks); err != nil {
		return &SteelThreadError{
			Message: fmt.Sprintf("cycle in steel thread: %v", err.Cycle),
		}
	}

	return nil
}

func ValidateDAG(tasks map[string]Task) ValidationResult {
	result := ValidationResult{Valid: true}

	if cycleErr := DetectCycles(tasks); cycleErr != nil {
		result.Valid = false
		result.Errors = append(result.Errors, cycleErr)
	}

	missingErrs := CheckDependencyExistence(tasks)
	for _, err := range missingErrs {
		result.Valid = false
		errCopy := err
		result.Errors = append(result.Errors, &errCopy)
	}

	if steelErr := ValidateSteelThread(tasks); steelErr != nil {
		result.Valid = false
		result.Errors = append(result.Errors, steelErr)
	}

	return result
}

func TopologicalSort(tasks map[string]Task) ([]string, error) {
	if cycleErr := DetectCycles(tasks); cycleErr != nil {
		return nil, cycleErr
	}

	inDegree := make(map[string]int)
	for id := range tasks {
		inDegree[id] = 0
	}
	for _, task := range tasks {
		for _, dep := range task.DependsOn {
			if _, exists := tasks[dep]; exists {
				inDegree[task.ID]++
			}
		}
	}

	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		sort.Strings(queue)
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for _, task := range tasks {
			for _, dep := range task.DependsOn {
				if dep == current {
					inDegree[task.ID]--
					if inDegree[task.ID] == 0 {
						queue = append(queue, task.ID)
					}
				}
			}
		}
	}

	return result, nil
}
