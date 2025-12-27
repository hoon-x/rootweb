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

package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type syncLogger struct {
	fileLogger *lumberjack.Logger
	zapLogger  *zap.Logger
}

var logger syncLogger

// InitializeLogger 로거를 초기화하고 파일 및 콘솔 출력 설정 구성
func InitializeLogger(logPath string, maxSize, maxBackups, maxAge int, compress, debug bool) {
	var cores []zapcore.Core

	// Lumberjack 설정: 로그 파일의 로테이션(용량 제한, 보관 기간 등)을 관리
	logger.fileLogger = &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}

	// 기본 인코더 설정: 로그의 출력 형식(레이아웃)을 정의
	baseEncoderConfig := zapcore.EncoderConfig{
		MessageKey:       "msg",
		LevelKey:         "level",
		TimeKey:          "time",
		StacktraceKey:    "stacktrace",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      capitalLevelEncoder,
		EncodeTime:       zapcore.TimeEncoderOfLayout("[2006-01-02 15:04:05]"),
		EncodeDuration:   zapcore.SecondsDurationEncoder,
		ConsoleSeparator: " ",
	}

	// 파일 출력 코어(Core) 설정
	// 파일에는 Caller(호출 위치) 정보를 남기지 않기 위해 기본 설정을 그대로 사용
	fileEncoder := zapcore.NewConsoleEncoder(baseEncoderConfig)
	fileWriter := zapcore.AddSync(logger.fileLogger)
	// Info 레벨 이상의 로그(Info, Warn, Error 등)만 파일에 저장
	cores = append(cores, zapcore.NewCore(fileEncoder, fileWriter, zapcore.InfoLevel))

	// 디버그 모드 전용 콘솔 출력 설정
	if debug {
		// 디버그용 별도 인코더 설정: 여기에서만 Caller 정보를 활성화
		debugEncoderConfig := baseEncoderConfig
		debugEncoderConfig.CallerKey = "caller"
		debugEncoderConfig.EncodeCaller = shortCallerEncoder
		debugEncoder := zapcore.NewConsoleEncoder(debugEncoderConfig)

		// 필터링: 오직 Debug 레벨의 로그만 통과
		onlyDebugLevel := zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return l == zapcore.DebugLevel
		})

		// 콘솔(표준 출력)로 내보내는 코어 추가
		consoleWriter := zapcore.AddSync(os.Stdout)
		cores = append(cores, zapcore.NewCore(debugEncoder, consoleWriter, onlyDebugLevel))
	}

	// 모든 코어를 하나로 결합 (Tee: 여러 출력처로 로그를 동시에 보냄)
	core := zapcore.NewTee(cores...)

	// 최종 로거 생성
	logger.zapLogger = zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.PanicLevel))
}

// FinalizeLogger 로거 자원 정리
func FinalizeLogger() {
	logger.zapLogger.Sync()
	logger.fileLogger.Close()
}

// capitalLevelEncoder zapcore의 CapitalLevelEncoder() 메서드 커스터마이징
func capitalLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("[" + l.CapitalString() + "]")
}

// shortCallerEncoder zapcore의 ShortCallerEncoder() 메서드 커스터마이징
func shortCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("[" + caller.TrimmedPath() + "]")
}

// LogDebug DEBUG 레벨 로깅 (콘솔 출력만)
func LogDebug(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logger.zapLogger.Debug(message)
}

// LogInfo INFO 레벨 로깅
func LogInfo(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logger.zapLogger.Info(message)
}

// LogWarn WARNING 레벨 로깅
func LogWarn(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logger.zapLogger.Warn(message)
}

// LogError ERROR 레벨 로깅
func LogError(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logger.zapLogger.Error(message)
}

// LogPanic PANIC 레벨 로깅
// 주의: Panic 발생됨
func LogPanic(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logger.zapLogger.Panic(message)
}

// LogFatal FATAL 레벨 로깅
// 주의: os.Exit(1) 실행됨
func LogFatal(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logger.zapLogger.Fatal(message)
}
