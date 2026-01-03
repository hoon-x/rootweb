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
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
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

	// 쿠키 세션 설정
	store := cookie.NewStore([]byte("rB9xQ7KfA2mW4ZP8EJcD6VtY5SgHnU3L"))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   1800,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// gin 라우터 생성
	r := gin.New()

	// HTML 템플릿 및 정적 리소스 설정
	r.LoadHTMLGlob("assets/templates/*.html")
	r.Static("/static", "./assets/static")

	// [미들웨어 정의]
	// TODO: 로그가 모듈 로그로 기록되도록 인터페이스 구현 필요
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	// 모든 접속 시 관리자 존재 여부 확인 미들웨어 등록
	r.Use(middleware.EnsureAdminExists())
	// 쿠키 세션 미들웨어 등록
	r.Use(sessions.Sessions("rootweb_sess", store))
	// 로그인 여부 확인 미들웨어 등록
	r.Use(middleware.RequireAuth())

	// [라우트 정의]
	// 초기 설정 핸들러 (관리자가 없을 때만 접근 가능)
	r.GET("/setup", handler.HtmlSetup)
	r.POST("/setup", handler.RegisterAdmin)
	// 로그인 처리 핸들러
	r.GET("/login", handler.HtmlLogin)
	r.POST("/login", handler.Login)
	r.GET("/logout", handler.Logout)
	// 터미널 처리 핸들러
	r.GET("/terminal", handler.HtmlTerminal)
	r.GET("/terminal/ws", handler.TerminalWS)
	r.GET("/ping", handler.Ping)
	// 메인 페이지 핸들러
	r.GET("/", handler.HtmlIndex)
	return r
}
