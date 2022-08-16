package task

import (
	"context"
	"fmt"

	"github.com/go-task/task/v3/internal/execext"
	"github.com/go-task/task/v3/internal/logger"
	"github.com/go-task/task/v3/internal/status"
	"github.com/go-task/task/v3/taskfile"
)

// Status returns an error if any the of given tasks is not up-to-date
func (e *Executor) Status(ctx context.Context, calls ...taskfile.Call) error {
	for _, call := range calls {
		t, err := e.CompiledTask(call)
		if err != nil {
			return err
		}
		isUpToDate, err := e.isTaskUpToDate(ctx, t)
		if err != nil {
			return err
		}
		if !isUpToDate {
			return fmt.Errorf(`task: Task "%s" is not up-to-date`, t.Name())
		}
	}
	return nil
}

// Status returns an error if any the of given tasks is not up-to-date

func (e *Executor) CreateTaskDAG(ctx context.Context, taskStr map[string][]string, aretaskUpToDate map[string]bool, calls ...taskfile.Call) (map[string][]string, map[string]bool) {
	var dep_calls []taskfile.Call
	for _, call := range calls {
		t, err := e.CompiledTask(call)
		if err != nil {
			// return err
			fmt.Println(err)
		}

		isUpToDate, err := e.isTaskUpToDate(ctx, t)
		taskName := t.Task
		if _, ok := taskStr[taskName]; !ok {
			taskStr[taskName] = make([]string, 1)
		}

		if len(t.Deps) == 0 {
			aretaskUpToDate[taskName] = isUpToDate
		} else {
			for _, d := range t.Deps {
				taskStr[taskName] = append(taskStr[taskName], d.Task)
				dep_calls = append(dep_calls, taskfile.Call{Task: d.Task, Vars: d.Vars})
			}
			taskStr, aretaskUpToDate = e.CreateTaskDAG(ctx, taskStr, aretaskUpToDate, dep_calls...)
		}
	}

	return taskStr, aretaskUpToDate
}

func (e *Executor) CheckStatusOfTaskAndDeps(ctx context.Context, aretaskUpToDate map[string]bool, calls ...taskfile.Call) map[string]bool {
	var dep_calls []taskfile.Call
	for _, call := range calls {
		t, err := e.CompiledTask(call)
		if err != nil {
			// return err
			fmt.Println(err)
		}

		taskName := t.Task
		isUpToDate, err := e.isTaskUpToDate(ctx, t)
		if err != nil {
			// return err
			fmt.Println(err)
		}
		aretaskUpToDate[taskName] = isUpToDate

		if len(t.Deps) != 0 {
			for _, d := range t.Deps {
				dep_calls = append(dep_calls, taskfile.Call{Task: d.Task, Vars: d.Vars})
				aretaskUpToDate = e.CheckStatusOfTaskAndDeps(ctx, aretaskUpToDate, dep_calls...)
				if !aretaskUpToDate[d.Task] {
					aretaskUpToDate[taskName] = false
				}
			}
		}

	}

	return aretaskUpToDate
}

func (e *Executor) isTaskUpToDate(ctx context.Context, t *taskfile.Task) (bool, error) {
	if len(t.Status) == 0 && len(t.Sources) == 0 {
		return false, nil
	}

	if len(t.Status) > 0 {
		isUpToDate, err := e.isTaskUpToDateStatus(ctx, t)
		if err != nil {
			return false, err
		}
		if !isUpToDate {
			return false, nil
		}
	}

	if len(t.Sources) > 0 {
		checker, err := e.getStatusChecker(t)
		if err != nil {
			return false, err
		}
		isUpToDate, err := checker.IsUpToDate()
		if err != nil {
			return false, err
		}
		if !isUpToDate {
			return false, nil
		}
	}

	return true, nil
}

func (e *Executor) statusOnError(t *taskfile.Task) error {
	checker, err := e.getStatusChecker(t)
	if err != nil {
		return err
	}
	return checker.OnError()
}

func (e *Executor) getStatusChecker(t *taskfile.Task) (status.Checker, error) {
	method := t.Method
	if method == "" {
		method = e.Taskfile.Method
	}
	switch method {
	case "timestamp":
		return e.timestampChecker(t), nil
	case "checksum":
		return e.checksumChecker(t), nil
	case "none":
		return status.None{}, nil
	default:
		return nil, fmt.Errorf(`task: invalid method "%s"`, method)
	}
}

func (e *Executor) timestampChecker(t *taskfile.Task) status.Checker {
	return &status.Timestamp{
		Dir:       t.Dir,
		Sources:   t.Sources,
		Generates: t.Generates,
	}
}

func (e *Executor) checksumChecker(t *taskfile.Task) status.Checker {
	return &status.Checksum{
		TempDir:   e.TempDir,
		TaskDir:   t.Dir,
		Task:      t.Name(),
		Sources:   t.Sources,
		Generates: t.Generates,
		Dry:       e.Dry,
	}
}

func (e *Executor) isTaskUpToDateStatus(ctx context.Context, t *taskfile.Task) (bool, error) {
	for _, s := range t.Status {
		err := execext.RunCommand(ctx, &execext.RunCommandOptions{
			Command: s,
			Dir:     t.Dir,
			Env:     getEnviron(t),
		})
		if err != nil {
			e.Logger.VerboseOutf(logger.Yellow, "task: status command %s exited non-zero: %s", s, err)
			return false, nil
		}
		e.Logger.VerboseOutf(logger.Yellow, "task: status command %s exited zero", s)
	}
	return true, nil
}
