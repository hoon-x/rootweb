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

package ipc

import (
	"context"
	"fmt"
	"sync"
	"syscall"

	"github.com/hoon-x/rootweb/config"
	"github.com/hoon-x/rootweb/internal/logger"
	"github.com/hoon-x/rootweb/pkg/proc"
)

type EvtType int

const (
	Shutdown EvtType = iota
)

type IpcManager struct {
	mu            sync.RWMutex
	ipcChan       chan EvtType
	isChanEnabled bool
}

var IpcMgr IpcManager

func init() {
	IpcMgr.ipcChan = make(chan EvtType, 16)
	IpcMgr.isChanEnabled = true
}

// Run 내부 통신 이벤트 처리 메서드
func Run(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		// 이벤트 처리
		for evt := range IpcMgr.ipcChan {
			switch evt {
			case Shutdown:
				logger.LogInfo("Shutdown event received")
				// 현재 프로세스에 종료 시그널 전송
				proc.SendSignal(config.RunConf.Pid, syscall.SIGTERM)
			}
		}
	}()

	// 고루틴 종료 이벤트 감지
	<-ctx.Done()

	// 채널 비활성화
	IpcMgr.mu.Lock()
	IpcMgr.isChanEnabled = false
	close(IpcMgr.ipcChan)
	IpcMgr.mu.Unlock()

	wg.Wait()
}

// SendIpcEvt IPC 이벤트 전송
func SendIpcEvt(evt EvtType) error {
	IpcMgr.mu.RLock()
	defer IpcMgr.mu.RUnlock()

	if !IpcMgr.isChanEnabled {
		return fmt.Errorf("ipc event channel is closed")
	}

	IpcMgr.ipcChan <- evt
	return nil
}
