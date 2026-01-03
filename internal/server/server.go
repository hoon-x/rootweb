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

package server

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hoon-x/rootweb/config"
	"github.com/hoon-x/rootweb/internal/db"
	"github.com/hoon-x/rootweb/internal/ipc"
	"github.com/hoon-x/rootweb/internal/logger"
	"github.com/hoon-x/rootweb/internal/router"
	"github.com/hoon-x/rootweb/pkg/cert"
	"github.com/hoon-x/rootweb/pkg/file"
)

// Run 서버 가동
func Run(ctx context.Context) {
	var once sync.Once
	var tlsConf tls.Config

	shutdown := func() {
		once.Do(func() {
			ipc.SendIpcEvt(ipc.Shutdown)
		})
	}

	// 서버 종료 시 프로세스가 종료될 수 있도록 함
	defer shutdown()

	// DB 경로 생성
	if err := os.MkdirAll(filepath.Dir(config.Conf.DB.DBPath), 0700); err != nil {
		logger.LogError("Failed to create directory (%s): %v", filepath.Dir(config.Conf.DB.DBPath), err)
		return
	}

	// DB 초기화
	if err := db.InitSqliteDB(config.Conf.DB.DBPath); err != nil {
		logger.LogError("Failed to initialize DB: %v", err)
		return
	}
	defer db.CloseSqliteDB()

	// TLS 인증서 파일이 없으면 새로 생성
	if !file.IsFileExists(config.Conf.Server.TlsCertPath) || !file.IsFileExists(config.Conf.Server.TlsKeyPath) {
		// TLS 인증서 파일 경로 생성
		if err := os.MkdirAll(filepath.Dir(config.Conf.Server.TlsCertPath), 0700); err != nil {
			logger.LogError("Failed to create directory (%s): %v", filepath.Dir(config.Conf.Server.TlsCertPath), err)
			return
		}

		// TLS 인증서 파일 생성
		err := cert.GenTLSCertificate(config.Conf.Server.TlsCertPath, config.Conf.Server.TlsKeyPath,
			config.ModuleName, 365)
		if err != nil {
			logger.LogError("Failed to create TLS certificate: %v", err)
			return
		}
	}

	// TLS 인증서 로드
	cert, err := tls.LoadX509KeyPair(config.Conf.Server.TlsCertPath, config.Conf.Server.TlsKeyPath)
	if err != nil {
		logger.LogError("Failed to load TLS certificate: %v", err)
		return
	}

	// TLS 인증서 등록
	tlsConf.Certificates = []tls.Certificate{cert}
	// 애플리케이션 계층 프로토콜(HTTP/1.1, HTTP/2) 설정
	tlsConf.NextProtos = []string{"h2", "http/1.1"}

	// 서버 설정
	server := &http.Server{
		Addr:           ":" + strconv.Itoa(config.Conf.Server.Port),
		Handler:        router.NewGinRouterEngine(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      &tlsConf,
	}

	go func() {
		// 서버 가동
		err := server.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			logger.LogError("Server error occurred: %v", err)
			shutdown()
		}
	}()

	// 종료 이벤트 감지
	<-ctx.Done()

	// 서버 종료 시 5초 타임아웃 설정
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 서버 종료
	err = server.Shutdown(shutdownCtx)
	if err != nil {
		logger.LogWarn("Failed to shutdown server: %v", err)
	}
}
