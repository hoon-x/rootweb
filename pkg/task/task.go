// Copyright 2025 JongHoon Shim
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package task

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

type TaskManager struct {
	panicHandler func(interface{})
	mu           sync.Mutex
	parentWG     sync.WaitGroup
	parentCtx    context.Context
	parentCancel context.CancelFunc
	tasks        map[string]*taskUnit
}

type taskUnit struct {
	childWG     sync.WaitGroup
	childCtx    context.Context
	childCancel context.CancelFunc
	task        func(ctx context.Context)
	isRun       bool
}

// NewTaskManager 작업 관리자 생성
func NewTaskManager(panicHandler func(interface{})) *TaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskManager{
		panicHandler: panicHandler,
		parentCtx:    ctx,
		parentCancel: cancel,
		tasks:        make(map[string]*taskUnit),
	}
}

// AddTask 작업 등록
func (tm *TaskManager) AddTask(name string, task func(ctx context.Context)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.tasks[name] = &taskUnit{
		task: task,
	}
}

// RemoveTask 등록된 작업 종료 및 제거
func (tm *TaskManager) RemoveTask(name string, timeout time.Duration) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 등록된 작업이 존재하는지 확인
	if t, exists := tm.tasks[name]; exists {
		if t.childCancel != nil {
			// 작업 종료 지시
			t.childCancel()
			// 작업 종료 대기
			if err := WaitGroupWithTimeout(&t.childWG, timeout); err != nil {
				return err
			}
		}
		delete(tm.tasks, name)
	}

	return nil
}

// RunAll 등록되어있는 모든 작업 가동
func (tm *TaskManager) RunAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, t := range tm.tasks {
		// 이미 동작 중인 작업은 무시
		if t.isRun {
			continue
		}

		// 자식 컨텍스트 생성
		ctx, cancel := context.WithCancel(tm.parentCtx)
		t.childCtx = ctx
		t.childCancel = cancel

		// Wait Group 등록
		tm.parentWG.Add(1)
		t.childWG.Add(1)

		t.isRun = true
		tmpTask := t

		go func(tu *taskUnit) {
			defer func() {
				// 패닉 발생 시 핸들러 설정
				if err := recover(); err != nil {
					if tm.panicHandler != nil {
						tm.panicHandler(err)
					} else {
						tm.defPanicHandler(err)
					}
				}
				tu.childWG.Done()
				tm.parentWG.Done()
			}()

			// 작업 실행
			tu.task(tu.childCtx)
		}(tmpTask)
	}
}

// Run 개별 작업 실행
func (tm *TaskManager) Run(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 작업이 등록되어 있는지 확인
	t, exists := tm.tasks[name]
	if !exists {
		return fmt.Errorf("task dose not exist (%s)", name)
	}

	// 이미 동작 중인 작업인 경우 무시
	if t.isRun {
		return nil
	}

	ctx, cancel := context.WithCancel(tm.parentCtx)
	t.childCtx = ctx
	t.childCancel = cancel

	tm.parentWG.Add(1)
	t.childWG.Add(1)

	t.isRun = true

	go func(tu *taskUnit) {
		defer func() {
			if err := recover(); err != nil {
				if tm.panicHandler != nil {
					tm.panicHandler(err)
				} else {
					tm.defPanicHandler(err)
				}
			}
			tu.childWG.Done()
			tm.parentWG.Done()
		}()

		tu.task(tu.childCtx)
	}(t)

	return nil
}

// ShutdownAll 동작 중인 모든 작업 종료
func (tm *TaskManager) ShutdownAll(timeout time.Duration) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 전체 작업 종료 지시
	for _, t := range tm.tasks {
		if t.childCancel != nil {
			t.childCancel()
		}
	}

	// 전체 작업 종료 대기
	if err := WaitGroupWithTimeout(&tm.parentWG, timeout); err != nil {
		return err
	}

	// 전체 작업 종료 성공 시 상태를 비활성화로 바꿔줌
	for _, t := range tm.tasks {
		t.isRun = false
	}

	return nil
}

// Shutdown 개별 작업 종료
func (tm *TaskManager) Shutdown(name string, timeout time.Duration) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t, exists := tm.tasks[name]; exists {
		if t.childCancel != nil {
			t.childCancel()
			if err := WaitGroupWithTimeout(&t.childWG, timeout); err != nil {
				return err
			}
		}
		t.isRun = false
	}

	return nil
}

// defPanicHandler 기본 패닉 에러 핸들러
func (tm *TaskManager) defPanicHandler(err interface{}) {
	fmt.Fprintf(os.Stderr, "panic occurred: %v\n", err)
}

// WaitGroupWithTimeout 고루틴 종료를 타임아웃만큼 대기하는 함수
func WaitGroupWithTimeout(wg *sync.WaitGroup, timeout time.Duration) error {
	if wg == nil {
		return fmt.Errorf("null parameter")
	}

	// 타임아웃이 0보다 작을 경우 무한 대기
	if timeout < 0 {
		wg.Wait()
		return nil
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout occurred")
	}
}
