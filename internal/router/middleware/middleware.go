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

package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/hoon-x/rootweb/internal/db"
)

var AdminExists bool

// EnsureAdminExists 관리자 계정 존재 여부 체크 미들웨어
func EnsureAdminExists() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 정적 파일 요청은 관리자 체크 로직을 타지 않고 즉시 통과
		if strings.HasPrefix(c.Request.URL.Path, "/static") || c.Request.URL.Path == "/favicon.ico" {
			c.Next()
			return
		}

		// 관리자 계정 존재 여부 체크 (메모리 캐시)
		if !AdminExists {
			// DB에서 관리자 계정 존재 여부 확인
			var exists bool
			err := db.SqliteDB.Model(&db.User{}).Select("1").Where("is_admin = ?", true).
				Limit(1).Find(&exists).Error
			if err == nil && exists {
				// 메모리 캐시 업데이트
				AdminExists = true
			}
		}

		if !AdminExists {
			// 관리자 계정이 없는데 /setup이 아닌 다른 경로로 간다면, /setup으로 강제 이동
			if c.Request.URL.Path != "/setup" {
				c.Redirect(http.StatusFound, "/setup")
				c.Abort()
				return
			}
		} else {
			// 관리자 계정이 있는데 /setup에 접근하려고 하면 /login 경로로 강제 이동
			if c.Request.URL.Path == "/setup" {
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// RequireAuth 로그인 되어있는지 확인하는 미들웨어
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		idleTimeout := 30 * time.Minute
		path := c.Request.URL.Path

		// 정적 리소스는 예외
		if strings.HasPrefix(path, "/static/") {
			c.Next()
			return
		}

		// 예외 경로 체크
		switch path {
		case "/login", "/setup", "/logout", "/favicon.ico":
			c.Next()
			return
		}

		// 세션 검사
		sess := sessions.Default(c)
		if sess.Get("user_id") == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// last_seen 읽기
		var lastSeenUnix int64
		if v := sess.Get("last_seen"); v != nil {
			switch t := v.(type) {
			case int64:
				lastSeenUnix = t
			case int:
				lastSeenUnix = int64(t)
			case float64:
				lastSeenUnix = int64(t)
			}
		}

		now := time.Now()
		lastSeen := time.Unix(lastSeenUnix, 0)

		// last_seen 없으면 비정상 세션으로 보고 재로그인 유도
		// IDLE 타임아웃 체크
		if lastSeenUnix == 0 || now.Sub(lastSeen) > idleTimeout {
			sess.Clear()
			sess.Options(sessions.Options{Path: "/", MaxAge: -1})
			_ = sess.Save()
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// 10분 마다 세션 유지 기간 갱신
		if now.Sub(lastSeen) >= 10*time.Minute {
			sess.Set("last_seen", now.Unix())
			sess.Options(sessions.Options{
				Path:     "/",
				MaxAge:   int(idleTimeout.Seconds()),
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
			})
			_ = sess.Save()
		}
		c.Next()
	}
}
