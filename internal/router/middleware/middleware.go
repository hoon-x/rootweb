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
