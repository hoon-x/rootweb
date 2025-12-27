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

package proc

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// IsProcRun PID로부터 프로세스가 동작 중인지 확인하는 함수
func IsProcRun(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// GetProcNameByPid PID로부터 프로세스명 추출
// 주의: 15자 까지만 추출 가능
func GetProcNameByPid(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// SendSignal PID를 가진 프로세스에 시그널 전송
func SendSignal(pid int, sig syscall.Signal) error {
	// PID로부터 프로세스를 찾음
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// 시그널 전송
	err = proc.Signal(sig)
	if err != nil {
		return err
	}

	return nil
}
