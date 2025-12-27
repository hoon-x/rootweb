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

package file

import "os"

// IsFileExists 주어진 경로에 파일이 존재하는지 확인하는 함수
func IsFileExists(filePath string) bool {
	stat, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return !stat.IsDir()
}
