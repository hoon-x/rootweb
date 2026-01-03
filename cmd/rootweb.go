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

package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/hoon-x/rootweb/config"
	"github.com/hoon-x/rootweb/internal/ipc"
	"github.com/hoon-x/rootweb/internal/logger"
	"github.com/hoon-x/rootweb/internal/server"
	"github.com/hoon-x/rootweb/pkg/proc"
	"github.com/hoon-x/rootweb/pkg/task"
	"github.com/spf13/cobra"
	"go.uber.org/automaxprocs/maxprocs"
)

var rootCmd = &cobra.Command{
	Use:   config.ModuleName,
	Short: "Modern Web-to-Terminal Gateway with Multi-Factor Authentication",
	Long: `
██████╗  ██████╗  ██████╗ ████████╗██╗    ██╗███████╗██████╗ 
██╔══██╗██╔═══██╗██╔═══██╗╚══██╔══╝██║    ██║██╔════╝██╔══██╗
██████╔╝██║   ██║██║   ██║   ██║   ██║ █╗ ██║█████╗  ██████╔╝
██╔══██╗██║   ██║██║   ██║   ██║   ██║███╗██║██╔══╝  ██╔══██╗
██║  ██║╚██████╔╝╚██████╔╝   ██║   ╚███╔███╔╝███████╗██████╔╝
╚═╝  ╚═╝ ╚═════╝  ╚═════╝    ╚═╝    ╚══╝╚══╝ ╚══════╝╚═════╝ 

RootWeb is a secure, high-performance Linux administration tool 
that provides a seamless bridge between your web browser and 
the native system terminal.

Core Capabilities:
  - Full-duplex Web Terminal via xterm.js & PTY
  - Hardened Security with Argon2 & TOTP (2FA)
  - Native Linux Daemon Management
  - Automatic TLS Certificate Provisioning`,
	Version: config.Version + ", Build Date: " + config.BuildDate + ", Commit: " + config.Commit,
}
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Run " + config.ModuleName + " (mode: normal)",
	RunE:  wrapCmdFuncForCobra(run),
}
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Run " + config.ModuleName + " (mode: debug)",
	RunE:  wrapCmdFuncForCobra(run),
}
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Shutdown " + config.ModuleName,
	RunE:  wrapCmdFuncForCobra(shutdown),
}

var taskManager *task.TaskManager

func init() {
	rootCmd.AddCommand(startCmd, debugCmd, stopCmd)
}

// Execute 프로그램 진입점 역할을 수행하며, 설정된 모든 명령어 실행
func Execute() {
	// 컨테이너(Docker/K8s) 환경의 CPU 할당량에 맞춰 GOMAXPROCS를 자동으로 최적화
	undo, err := maxprocs.Set()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Failed to set GOMAXPROCS: %v\n", err)
	}
	defer undo()

	// 사용자가 터미널에 입력한 인자를 분석하여 해당하는 하위 명령어 실행
	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// wrapCmdFuncForCobra cobra.Command의 RunE 함수 원형에 맞게 기존 비즈니스 로직 함수를 감싸는(Wrap) 헬퍼 함수
func wrapCmdFuncForCobra(f func(cmd *cobra.Command) error) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		// cobra에서 출력하는 에러 메시지 무시
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return f(cmd)
	}
}

// run 모듈 가동
func run(cmd *cobra.Command) error {
	// 작업 경로를 실행 파일이 위치한 경로로 변경
	if err := chdirToExecutableDir(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to change working path: %v\n", err)
		return err
	}

	// 이미 동작 중인지 확인
	var pid int
	if isRun(&pid, config.PidFilePath) {
		fmt.Fprintf(os.Stderr, "[INFO] %s is already running (pid:%d)\n", config.ModuleName, pid)
		return nil
	}

	if cmd.Use == "debug" {
		config.RunConf.Debug = true
	} else {
		// 프로세스 데몬화
		if err := daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to daemonize process: %v\n", err)
			return err
		}
		// 파일이나 디렉터리 생성 시 적용되는 기본 권한 마스크 제거
		syscall.Umask(0)
	}

	// PID 정보 저장
	config.RunConf.Pid = os.Getpid()

	// `var` 디렉터리 생성
	if err := os.MkdirAll("var", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to create directory: %v\n", err)
		return err
	}

	// 현재 PID를 파일에 씀
	err := os.WriteFile(config.PidFilePath, []byte(strconv.Itoa(config.RunConf.Pid)), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to write PID file: %v\n", err)
		return err
	}

	// 시그널 설정
	sigChan := setSignal()
	defer signal.Stop(sigChan)

	// 설정 파일 로드
	if err := config.Conf.LoadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to load config: %v\n", err)
		return err
	}

	// 표준 입출력, 에러를 /dev/null로 리다이렉트
	if err := redirectToDevNull(config.RunConf.Debug); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to redirect stdin,stdout,stderr to /dev/null: %v\n", err)
		return err
	}

	// 모듈 초기화
	initialize()
	logger.LogInfo("Run %s (pid:%d)", config.ModuleName, config.RunConf.Pid)

	// 등록된 모든 작업 가동
	taskManager.RunAll()

	// 종료 시그널 대기 (SIGINT, SIGTERM)
	sig := <-sigChan

	logger.LogInfo("Received %s (signum:%d)", sig.String(), sig)

	// 모듈 자원 정리
	finalize()

	return nil
}

// shutdown 모듈 정지
func shutdown(cmd *cobra.Command) error {
	// 작업 경로를 실행 파일이 위치한 경로로 변경
	err := chdirToExecutableDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to change working path: %v\n", err)
		return err
	}

	// 프로세스가 동작 중인지 확인
	var pid int
	if !isRun(&pid, config.PidFilePath) {
		return nil
	}

	// 프로세스에 종료 시그널 전송
	err = proc.SendSignal(pid, syscall.SIGTERM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to send signal (pid:%d): %v\n", pid, err)
		return err
	}

	return nil
}

// chdirToExecutableDir 프로세스 작업 경로를 실행 파일이 위치한 경로로 변경
func chdirToExecutableDir() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	dirPath := filepath.Dir(exePath)

	err = os.Chdir(dirPath)
	if err != nil {
		return err
	}

	return nil
}

// isRun 프로세스가 이미 동작 중인지 확인하는 함수
func isRun(pid *int, pidFilePath string) bool {
	// PID 파일 오픈
	file, err := os.Open(pidFilePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// PID 읽음
	pidStr, err := io.ReadAll(file)
	if err != nil {
		return false
	}

	// 문자열을 숫자로 변환
	tmpPid, err := strconv.Atoi(string(pidStr))
	if err != nil {
		return false
	}

	if pid != nil {
		*pid = tmpPid
	}

	// 프로세스가 동작 중인지 확인
	if !proc.IsProcRun(tmpPid) {
		return false
	}

	// 프로세스명 추출하여 모듈명과 일치하는지 확인
	procName, err := proc.GetProcNameByPid(tmpPid)
	if err != nil || procName != config.ModuleName {
		return false
	}

	return true
}

// daemonize 프로세스 데몬화
func daemonize() error {
	// 이미 데몬 프로세스인지 확인
	if os.Getppid() == 1 || os.Getenv("IS_DAEMON") == "1" {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	// 자식 프로세스 설정
	cmd := exec.Command(exePath, os.Args[1:]...)
	cmd.Env = append(os.Environ(), "IS_DAEMON=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 자식 프로세스 실행
	err = cmd.Start()
	if err != nil {
		return err
	}

	// 부모 프로세스 종료
	os.Exit(0)

	return nil
}

// redirectToDevNull 표준 출력/에러를 /dev/null로 리다이렉트
func redirectToDevNull(debug bool) error {
	file, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := syscall.Dup2(int(file.Fd()), int(os.Stdin.Fd())); err != nil {
		return err
	}
	os.Stdin = file
	if !debug {
		if err := syscall.Dup2(int(file.Fd()), int(os.Stdout.Fd())); err != nil {
			return err
		}
		if err := syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd())); err != nil {
			return err
		}
	}

	return nil
}

// setSignal 시그널 설정
func setSignal() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	// 수신할 시그널 설정 (SIGINT, SIGTERM)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// 무시할 시그널 설정
	signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE, syscall.SIGTTIN, syscall.SIGTTOU,
		syscall.SIGTSTP, syscall.SIGQUIT, syscall.SIGWINCH, syscall.SIGURG)

	return sigChan
}

// initialize 모듈 초기화
func initialize() {
	// 로거 초기화
	logger.InitializeLogger(config.LogFilePath,
		config.Conf.Log.MaxSize, config.Conf.Log.MaxBackups,
		config.Conf.Log.MaxAge, config.Conf.Log.Compress,
		config.RunConf.Debug)

	// 작업 관리자 생성
	taskManager = task.NewTaskManager(panicHandler)

	// 서버 작업 등록
	taskManager.AddTask("server", server.Run)
	taskManager.AddTask("ipc_manager", ipc.Run)
}

// finalize 모듈 자원 정리
func finalize() {
	// 가동중인 모든 작업 종료 지시
	if err := taskManager.ShutdownAll(10 * time.Second); err != nil {
		logger.LogWarn("All tasks have not been completed: %v", err)
	}
	logger.LogInfo("Shutdown %s (pid:%d)", config.ModuleName, config.RunConf.Pid)

	// 로거 자원 해제
	logger.FinalizeLogger()
}

// panicHandler 고루틴 패닉 핸들러
func panicHandler(err interface{}) {
	logger.LogError("panic occurred: %v", err)
}
