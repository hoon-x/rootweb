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

package handler

import (
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoon-x/rootweb/internal/db"
	"github.com/hoon-x/rootweb/internal/router/middleware"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// HtmlSetup [GET /setup] 최초 접속 시 관리자 계정 setup 페이지
func HtmlSetup(c *gin.Context) {
	// 관리자 계정이 존재하면 바로 /login 경로로 리다이렉트
	if middleware.AdminExists {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Google OTP용 Secret 생성
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "RootWeb",
		AccountName: "admin@rootweb",
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "OTP 생성 중 오류가 발생했습니다.")
		return
	}

	// QR코드 생성
	var png []byte
	png, err = qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		c.String(http.StatusInternalServerError, "QR 생성 실패")
		return
	}
	qrBase64 := base64.StdEncoding.EncodeToString(png)

	// 템플릿 렌더링
	// Secret: 폼 제출 시 다시 받아야 하므로 hidden input용
	// QRURL: 구글 차트 API가 QR 이미지를 생성할 수 있도록 넘겨주는 주소
	c.HTML(http.StatusOK, "setup.html", gin.H{
		"Secret":   key.Secret(),
		"QRBase64": qrBase64,
	})
}

// RegisterAdmin [POST /setup] 관리자 계정 등록
func RegisterAdmin(c *gin.Context) {
	// 폼 데이터 수집
	username := c.PostForm("username")
	password := c.PostForm("password")
	secret := c.PostForm("secret")
	token := c.PostForm("otp_token")

	// 동시 요청으로 인한 중복 생성 방지
	if middleware.AdminExists {
		c.JSON(http.StatusForbidden, gin.H{"error": "관리자가 이미 존재합니다."})
		return
	}

	// OTP 번호 유효성 검증
	if !totp.Validate(token, secret) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP 인증 번호가 일치하지 않습니다."})
		return
	}

	// 비밀번호 보안 해싱
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "비밀번호 암호화 실패"})
		return
	}

	// 관리자 계정 생성
	newAdmin := db.User{
		Username:  username,
		Password:  string(hashedPassword),
		OTPSecret: secret,
		IsAdmin:   true,
	}

	// DB에 관리자 계정 등록
	err = db.SqliteDB.Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&db.User{}).Where("is_admin = ?", true).Count(&count)
		if count > 0 {
			return errors.New("already_initialized")
		}
		return tx.Create(&newAdmin).Error
	})
	if err != nil {
		if err.Error() == "already_initialized" {
			c.JSON(http.StatusForbidden, gin.H{"error": "관리자가 이미 존재합니다."})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "관리자 계정 저장 실패"})
		}
		return
	}

	// 전역 캐시 업데이트 (이제부터 모든 미들웨어는 DB 조회 없이 통과)
	middleware.AdminExists = true

	// 설정 완료 후 로그인 페이지로 이동
	c.Redirect(http.StatusFound, "/login")
}
