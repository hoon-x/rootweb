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

package config

import (
	"os"

	"go.yaml.in/yaml/v2"
)

// 빌드 시 값 설정됨
var (
	ModuleName = "unknown"
	Version    = "unknown"
	BuildDate  = "unknown"
	Commit     = "unknown"
)

var (
	LogFilePath  = "log/" + ModuleName + ".log"
	ConfFilePath = "config/" + ModuleName + ".yaml"
	PidFilePath  = "var/." + ModuleName + ".pid"
)

type Config struct {
	// 서버 설정
	Server struct {
		// 서버 활성화 플래그
		Enabled bool `yaml:"enabled"`
		// 서버 리스닝 포트
		Port int `yaml:"port"`
		// TLS 인증서 파일 경로
		TlsCertPath string `yaml:"tlsCertPath"`
		TlsKeyPath  string `yaml:"tlsKeyPath"`
	} `yaml:"server"`

	// DB 설정
	DB struct {
		DBPath string `yaml:"dbPath"`
	} `yaml:"db"`

	// 로그 설정
	Log struct {
		// 최대 로그 파일 사이즈 (단위:MB)
		MaxSize int `yaml:"maxSize"`
		// 최대 로그 파일 백업 개수
		MaxBackups int `yaml:"maxBackups"`
		// 백업 로그 파일 최대 유지 기간 (단위:일)
		MaxAge int `yaml:"maxAge"`
		// 로그 파일 백업 시 압축 여부
		Compress bool `yaml:"compress"`
	} `yaml:"log"`
}

type RunConfig struct {
	Debug bool
	Pid   int
}

var Conf Config
var RunConf RunConfig

// LoadConfig YAML 설정 파일 로드
func (c *Config) LoadConfig() error {
	// YAML 설정 파일 오픈
	file, err := os.Open(ConfFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// YAML 디코더 생성
	decoder := yaml.NewDecoder(file)

	// YAML 파싱
	err = decoder.Decode(c)
	if err != nil {
		return err
	}

	return nil
}
