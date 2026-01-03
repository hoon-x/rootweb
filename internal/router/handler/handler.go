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
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hoon-x/rootweb/internal/db"
	"github.com/hoon-x/rootweb/internal/logger"
	"github.com/hoon-x/rootweb/internal/router/middleware"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type wsMsg struct {
	MsgType string `json:"type"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// HtmlSetup [GET /setup] 최초 접속 시 관리자 계정 /setup 페이지 렌더링
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
		logger.LogError("Failed to create OTP: IP=%s, err=%v", c.ClientIP(), err)
		return
	}

	// QR코드 생성
	var png []byte
	png, err = qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		c.String(http.StatusInternalServerError, "QR 생성 실패")
		logger.LogError("Failed to create QR: IP=%s, err=%v", c.ClientIP(), err)
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
		logger.LogError("Failed to hash password: IP=%s, err=%v", c.ClientIP(), err)
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
			logger.LogError("Failed to store admin account: IP=%s, err=%v", c.ClientIP(), err)
		}
		return
	}

	// 전역 캐시 업데이트 (이제부터 모든 미들웨어는 DB 조회 없이 통과)
	middleware.AdminExists = true

	// 설정 완료 후 로그인 페이지로 이동
	c.Redirect(http.StatusFound, "/login")
}

// HtmlLogin [GET /login] 로그인 페이지 렌더링
func HtmlLogin(c *gin.Context) {
	// 이미 로그인 상태이면 / 경로로 보냄
	sess := sessions.Default(c)
	if sess.Get("user_id") != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}
	c.HTML(http.StatusOK, "login.html", gin.H{"Error": ""})
}

// Login [POST /login] 로그인 처리
func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	otpToken := c.PostForm("otp_token") // 추가: 폼에서 otp_token으로 보내기

	// ID 조회
	var admin db.User
	if err := db.SqliteDB.Where("username = ? AND is_admin = ?", username, true).First(&admin).Error; err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "아이디 또는 비밀번호가 올바르지 않습니다."})
		return
	}

	// PW 검증
	if bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password)) != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "아이디 또는 비밀번호가 올바르지 않습니다."})
		return
	}

	// OTP 검증
	if !totp.Validate(otpToken, admin.OTPSecret) {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "OTP 인증 번호가 일치하지 않습니다."})
		return
	}

	// 세션 저장
	sess := sessions.Default(c)
	sess.Clear()
	sess.Set("user_id", admin.ID)
	now := time.Now().Unix()
	sess.Set("last_seen", now)
	sess.Set("otp_verified_at", now)
	if err := sess.Save(); err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{"Error": "세션 저장 실패"})
		logger.LogError("Failed to save session info: IP=%s, err=%v", c.ClientIP(), err)
		return
	}

	c.Redirect(http.StatusFound, "/")
}

// Logout [GET /logout] 로그아웃 처리
func Logout(c *gin.Context) {
	sess := sessions.Default(c)
	sess.Clear()
	sess.Options(sessions.Options{MaxAge: -1, Path: "/"})
	sess.Save()
	c.Redirect(http.StatusFound, "/login")
}

// HtmlIndex [GET /] 메인 페이지 렌더링
func HtmlIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{})
}

// HtmlTerminal [GET /terminal] 터미널 페이지 렌더링
func HtmlTerminal(c *gin.Context) {
	c.HTML(http.StatusOK, "terminal.html", gin.H{})
}

// TerminalWS [GET /terminal/ws] 클라이언트와 서버 PTY 간의 웹소켓 브라우징 중
func TerminalWS(c *gin.Context) {
	// HTTP 연결을 웹소켓 프로토콜로 업그레이드 (Handshake)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.LogError("Failed to upgrade web socket: IP=%s path=%s origin=%q UA=%q err=%v",
			c.ClientIP(),
			c.Request.URL.Path,
			c.GetHeader("Origin"),
			c.Request.UserAgent(),
			err)
		return
	}

	// 실행할 쉘 설정 (기본 bash) 및 환경 변수/작업 디렉토리 지정
	cmd := exec.Command("/bin/bash")
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
	)
	cmd.Dir = "/root"

	// PTY(가상 터미널) 시작 및 쉘 실행
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: 120, Rows: 30})
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("failed to open pty"))
		logger.LogError("Failed to start PTY: IP=%s, err=%v", c.ClientIP(), err)
		conn.Close()
		return
	}

	// 동기화 및 종료 처리를 위한 채널들
	done1 := make(chan struct{})
	done2 := make(chan struct{})
	stopCh := make(chan struct{})

	// 중복 종료를 방지하기 위한 안전 장치 (sync.Once)
	var once sync.Once
	stop := func() {
		once.Do(func() {
			// stopCh를 닫아 대기 중인 메인 루프에 종료 알림
			close(stopCh)
		})
	}

	// --- 고루틴 A: 서버 PTY의 출력을 읽어서 클라이언트(웹브라우저)로 전달 ---
	go func() {
		defer close(done1)
		defer stop()
		buf := make([]byte, 8192) // 쉘로부터 출력 데이터 읽기
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				// 웹소켓 쓰기 타임아웃 설정 (네트워크 지연 시 무한 대기 방지)
				if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
					logger.LogError("Failed to set write deadline: IP=%s, err=%v", c.ClientIP(), err)
					return
				}

				// 읽은 데이터를 바이너리 형식으로 웹소켓 전송
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					logger.LogError("Failed to write message: IP=%s, err=%v", c.ClientIP(), err)
					return
				}
			}
			if rerr != nil {
				return
			}
		}
	}()

	// --- 고루틴 B: 클라이언트의 입력을 읽어서 서버 PTY로 전달 ---
	go func() {
		defer close(done2)
		defer stop()
		for {
			// 클라이언트로부터 웹소켓 메시지 수신
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				logger.LogError("Failed to read message: %v", err)
				return
			}

			// Case 1: 순수 터미널 입력 데이터 (바이너리)
			if msgType == websocket.BinaryMessage {
				if len(msg) > 0 {
					// 수신한 키 입력 등을 실제 쉘 프로세스(PTY)에 쓰기
					if _, err := ptmx.Write(msg); err != nil {
						logger.LogError("Failed to write PTY: IP=%s, err=%v", c.ClientIP(), err)
						return
					}
				}
			} else if msgType == websocket.TextMessage {
				// Case 2: 터미널 제어용 메시지 (JSON 형식, 예: 리사이즈)
				var r wsMsg
				if err := json.Unmarshal(msg, &r); err != nil {
					logger.LogWarn("Failed to Unmarshal: IP=%s, err=%v", c.ClientIP(), err)
					continue
				}
				// 브라우저 창 크기가 변했을 때 PTY 크기도 동기화
				if r.MsgType == "resize" {
					if r.Cols > 0 && r.Rows > 0 {
						pty.Setsize(ptmx, &pty.Winsize{
							Cols: uint16(r.Cols),
							Rows: uint16(r.Rows),
						})
					}
				}
			}
		}
	}()

	// 종료 처리 대기 (어느 한쪽 고루틴이라도 stop()을 호출하면 해제됨)
	<-stopCh
	// 자원 정리 (연결 해제 및 프로세스 종료)
	conn.Close()       // 웹소켓 닫기
	<-done2            // WS 읽기 고루틴 종료 대기
	ptmx.Close()       // PTY 디바이스 닫기
	cmd.Process.Kill() // 쉘 프로세스 강제 종료
	cmd.Process.Wait() // 좀비 프로세스 방지를 위한 상태 대기
	<-done1            // PTY 읽기 고루틴 종료 대기
}

// Ping [GET /ping] 세션 유지를 위한 단순 응답 핸들러
func Ping(c *gin.Context) {
	// 미들웨어에서 이미 세션 체크 및 last_seen 업데이트가 이루어짐
	c.JSON(200, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}
