/*
 * Copyright 2019 Grabtaxi Holdings PTE LTE (GRAB), All rights reserved.
 * Use of this source code is governed by an MIT-style license that can be found in the LICENSE file
 */

package symphony

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Symphony defines top struct
type Symphony struct {
	tasks map[string]*task
	err   error
}

// TaskState defines the state of each task
type TaskState struct {
	R interface{}
	E error
}

type taskFunc func(res map[string]*TaskState) (interface{}, error)

type task struct {
	Name string
	Deps []string
	Ctr  int
	Fn   taskFunc
	C    chan *TaskState
	once sync.Once
}

// done sends the task result
func (t *task) done(r interface{}, err error) {
	for i := 0; i < t.Ctr; i++ {
		t.C <- &TaskState{
			R: r,
			E: err,
		}
	}
}

// close the channel for the task
func (t *task) close() {
	t.once.Do(func() {
		close(t.C)
	})
}

func (t *task) initTask() {
	t.C = make(chan *TaskState, t.Ctr)
}

// New symphony
func New() *Symphony {
	return &Symphony{
		tasks: map[string]*task{},
	}
}

// Add adds tasks
func (symphony *Symphony) Add(name string, dep []string, fn taskFunc) *Symphony {
	if symphony.err != nil {
		return symphony
	}
	if _, existed := symphony.tasks[name]; existed {
		symphony.err = fmt.Errorf(`error: Duplicated Func Name "%s" found`, name)
		return symphony
	}
	symphony.tasks[name] = &task{
		Name: name,
		Deps: dep,
		Fn:   fn,
		Ctr:  1, // prevent deadlock
	}
	return symphony
}

// dfs check cyclic dependency, see https://en.wikipedia.org/wiki/Topological_sorting#Depth-first_search
func dfs(node string, visited map[string]int, symphony *Symphony, path []string) (bool, []string) {
	if visited[node] == 1 {
		return true, path // cyclic dependent
	}
	if visited[node] == 2 {
		return false, path
	}
	// 1 = temporarily visited
	visited[node] = 1
	path = append(path, node)
	deps := symphony.tasks[node].Deps
	for _, dep := range deps {
		if cyclic, path := dfs(dep, visited, symphony, path); cyclic {
			return true, path
		}
	}
	// 2 = permanently visited
	visited[node] = 2

	return false, path
}

func (symphony *Symphony) hasCyclicDep() (bool, []string) {
	var visited = map[string]int{}

	for name := range symphony.tasks {
		visited[name] = 0
	}
	for name, state := range visited {
		if state == 0 {
			if cyclic, path := dfs(name, visited, symphony, []string{}); cyclic {
				return true, path
			}
		}
	}
	return false, nil
}

// Do starts running the tasks
func (symphony *Symphony) Do(ctx context.Context, timeoutInMs int64) (map[string]*TaskState, error) {
	childCtx, cancelFunc := context.WithTimeout(ctx, time.Duration(timeoutInMs)*time.Millisecond)
	defer cancelFunc()

	if symphony.err != nil {
		return nil, symphony.err
	}

	for name, fn := range symphony.tasks {
		for _, dep := range fn.Deps {
			// prevent self depends
			if dep == name {
				return nil, fmt.Errorf(`error: Function "%s" depends of itself`, name)
			}
			// prevent no existing dependencies
			if _, exists := symphony.tasks[dep]; exists == false {
				return nil, fmt.Errorf(`error: Function "%s" not exists`, dep)
			}
			symphony.tasks[dep].Ctr++
		}
	}
	// check circular dependency
	if cyclic, path := symphony.hasCyclicDep(); cyclic {
		return nil, fmt.Errorf("error: Has cyclic dependency, #%v", strings.Join(path, " <- "))
	}

	doneC := make(chan struct{})
	res := make(map[string]*TaskState, len(symphony.tasks))
	go symphony.do(childCtx, res, doneC)
	select {
	case <-doneC:
		for _, ts := range res {
			if ts != nil && ts.E != nil {
				return res, ts.E
			}
		}
		return res, nil
	case <-childCtx.Done():
		return res, fmt.Errorf("error: Timeout, childCtx Err: %s", childCtx.Err())
	}
}

func (symphony *Symphony) do(ctx context.Context, res map[string]*TaskState, doneC chan struct{}) {

	for _, f := range symphony.tasks {
		f.initTask()
	}

	for name, f := range symphony.tasks {
		go func(name string, t *task) {
			defer func() { t.close() }()

			results := make(map[string]*TaskState, len(t.Deps))

			// drain dependency results
			for _, dep := range t.Deps {
				results[dep] = <-symphony.tasks[dep].C
				// if any error happens, stop all dependent tasks
				if results[dep].E != nil {
					t.done(nil, results[dep].E)
					return
				}
			}

			r, fnErr := t.Fn(results)

			if fnErr != nil {
				t.done(r, fnErr)
				return
			}
			t.done(r, nil)

		}(name, f)
	}

	// wait for all
	for name, fs := range symphony.tasks {
		res[name] = <-fs.C
	}
	close(doneC)
	return
}
