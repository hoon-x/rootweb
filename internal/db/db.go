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

package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username  string `gorm:"uniqueIndex;not null"`
	Password  string `gorm:"not null"`
	OTPSecret string `gorm:"default:null"`
	IsAdmin   bool   `gorm:"index;default:false;not null"`
}

var SqliteDB *gorm.DB

// InitSqliteDB SQLite DB 초기화
func InitSqliteDB(dbPath string) error {
	var err error
	SqliteDB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}
	SqliteDB.AutoMigrate(&User{})
	return nil
}

// CloseSqliteDB SQLite DB 연결 해제
func CloseSqliteDB() {
	if SqliteDB != nil {
		sqlDB, err := SqliteDB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}
