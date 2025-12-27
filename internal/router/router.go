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

package router

import (
	"github.com/gin-gonic/gin"
	"github.com/hoon-x/rootweb/config"
	"github.com/hoon-x/rootweb/internal/router/handler"
	"github.com/hoon-x/rootweb/internal/router/middleware"
)

// NewGinRouterEngine gin 프레임워크 엔진 생성
func NewGinRouterEngine() *gin.Engine {
	// gin 동작 모드 설정
	gin.SetMode(func() string {
		if config.RunConf.Debug {
			return gin.DebugMode
		}
		return gin.ReleaseMode
	}())

	// gin 라우터 생성
	r := gin.New()

	// HTML 템플릿 및 정적 리소스 설정
	r.LoadHTMLGlob("assets/templates/*.html")
	r.Static("/static", "./assets/static")

	// TODO: 로그가 모듈 로그로 기록되도록 인터페이스 구현 필요
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	// 모든 접속 시 관리자가 있는지 체크
	r.Use(middleware.EnsureAdminExists())

	// 라우트 정의
	// 초기 설정 (관리자가 없을 때만 접근 가능)
	r.GET("/setup", handler.HtmlSetup)
	r.POST("/setup", handler.RegisterAdmin)

	return r
}
