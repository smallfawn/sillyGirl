// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logs

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testLogFile(t *testing.T, name string) string {
	t.Helper()
	return filepath.ToSlash(filepath.Join(t.TempDir(), name))
}

func TestFilePerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permission bits are not reliable on Windows")
	}
	log := NewLogger(10000)
	filename := testLogFile(t, "test.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q, "perm": "0666"}`, filename))
	defer log.Close()
	log.Debug("debug")
	log.Informational("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()
	file, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if file.Mode().Perm() != 0o666 {
		t.Fatal("unexpected log file permission")
	}
}

func TestFileWithPrefixPath(t *testing.T) {
	log := NewLogger(10000)
	filename := testLogFile(t, filepath.Join("log", "test.log"))
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q}`, filename))
	defer log.Close()
	log.Debug("debug")
	log.Informational("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()
	_, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFilePermWithPrefixPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permission bits are not reliable on Windows")
	}
	log := NewLogger(10000)
	filename := testLogFile(t, filepath.Join("mylogpath", "test.log"))
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q, "perm": "0220", "dirperm": "0770"}`, filename))
	defer log.Close()
	log.Debug("debug")
	log.Informational("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()

	dir, err := os.Stat(filepath.Dir(filename))
	if err != nil {
		t.Fatal(err)
	}
	if !dir.IsDir() {
		t.Fatal("mylogpath expected to be a directory")
	}

	file, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if file.Mode().Perm() != 0o0220 {
		t.Fatal("unexpected file permission")
	}
}

func TestFile1(t *testing.T) {
	log := NewLogger(10000)
	filename := testLogFile(t, "test.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q}`, filename))
	defer log.Close()
	log.Debug("debug")
	log.Informational("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()
	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := bufio.NewReader(f)
	lineNum := 0
	for {
		line, _, err := b.ReadLine()
		if err != nil {
			break
		}
		if len(line) > 0 {
			lineNum++
		}
	}
	expected := LevelDebug + 1
	if lineNum != expected {
		t.Fatal(lineNum, "not "+strconv.Itoa(expected)+" lines")
	}
}

func TestFile2(t *testing.T) {
	log := NewLogger(10000)
	filename := testLogFile(t, "test2.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q,"level":%d}`, filename, LevelError))
	defer log.Close()
	log.Debug("debug")
	log.Info("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()
	f, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b := bufio.NewReader(f)
	lineNum := 0
	for {
		line, _, err := b.ReadLine()
		if err != nil {
			break
		}
		if len(line) > 0 {
			lineNum++
		}
	}
	expected := LevelError + 1
	if lineNum != expected {
		t.Fatal(lineNum, "not "+strconv.Itoa(expected)+" lines")
	}
}

func TestFileDailyRotate_01(t *testing.T) {
	log := NewLogger(10000)
	filename := testLogFile(t, "test3.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q,"maxlines":4}`, filename))
	defer log.Close()
	log.Debug("debug")
	log.Info("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()
	rotateName := filepath.Join(filepath.Dir(filename), "test3"+fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), 1)+".log")
	b, err := exists(rotateName)
	if !b || err != nil {
		t.Fatal("rotate not generated")
	}
}

func TestFileDailyRotate_02(t *testing.T) {
	dir := t.TempDir()
	fn1 := filepath.ToSlash(filepath.Join(dir, "rotate_day.log"))
	fn2 := filepath.ToSlash(filepath.Join(dir, "rotate_day."+time.Now().Add(-24*time.Hour).Format("2006-01-02")+".001.log"))
	testFileRotate(t, fn1, fn2, true, false)
}

func TestFileDailyRotate_03(t *testing.T) {
	dir := t.TempDir()
	fn1 := filepath.ToSlash(filepath.Join(dir, "rotate_day.log"))
	fn := filepath.ToSlash(filepath.Join(dir, "rotate_day."+time.Now().Add(-24*time.Hour).Format("2006-01-02")+".log"))
	file, _ := os.Create(fn)
	file.Close()
	fn2 := filepath.ToSlash(filepath.Join(dir, "rotate_day."+time.Now().Add(-24*time.Hour).Format("2006-01-02")+".001.log"))
	testFileRotate(t, fn1, fn2, true, false)
}

func TestFileDailyRotate_04(t *testing.T) {
	dir := t.TempDir()
	fn1 := filepath.ToSlash(filepath.Join(dir, "rotate_day.log"))
	fn2 := filepath.ToSlash(filepath.Join(dir, "rotate_day."+time.Now().Add(-24*time.Hour).Format("2006-01-02")+".001.log"))
	testFileDailyRotate(t, fn1, fn2)
}

func TestFileDailyRotate_05(t *testing.T) {
	dir := t.TempDir()
	fn1 := filepath.ToSlash(filepath.Join(dir, "rotate_day.log"))
	fn := filepath.ToSlash(filepath.Join(dir, "rotate_day."+time.Now().Add(-24*time.Hour).Format("2006-01-02")+".log"))
	file, _ := os.Create(fn)
	file.Close()
	fn2 := filepath.ToSlash(filepath.Join(dir, "rotate_day."+time.Now().Add(-24*time.Hour).Format("2006-01-02")+".001.log"))
	testFileDailyRotate(t, fn1, fn2)
}

func TestFileDailyRotate_06(t *testing.T) { // test file mode
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permission bits are not reliable on Windows")
	}
	log := NewLogger(10000)
	filename := testLogFile(t, "test3.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q,"maxlines":4}`, filename))
	defer log.Close()
	log.Debug("debug")
	log.Info("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()
	rotateName := filepath.Join(filepath.Dir(filename), "test3"+fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), 1)+".log")
	s, _ := os.Lstat(rotateName)
	if s.Mode().Perm() != 0o440 {
		t.Fatal("rotate file mode error")
	}
}

func TestFileHourlyRotate_01(t *testing.T) {
	log := NewLogger(10000)
	filename := testLogFile(t, "test3.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q,"hourly":true,"maxlines":4}`, filename))
	defer log.Close()
	log.Debug("debug")
	log.Info("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()
	rotateName := filepath.Join(filepath.Dir(filename), "test3"+fmt.Sprintf(".%s.%03d", time.Now().Format("2006010215"), 1)+".log")
	b, err := exists(rotateName)
	if !b || err != nil {
		t.Fatal("rotate not generated")
	}
}

func TestFileHourlyRotate_02(t *testing.T) {
	dir := t.TempDir()
	fn1 := filepath.ToSlash(filepath.Join(dir, "rotate_hour.log"))
	fn2 := filepath.ToSlash(filepath.Join(dir, "rotate_hour."+time.Now().Add(-1*time.Hour).Format("2006010215")+".001.log"))
	testFileRotate(t, fn1, fn2, false, true)
}

func TestFileHourlyRotate_03(t *testing.T) {
	dir := t.TempDir()
	fn1 := filepath.ToSlash(filepath.Join(dir, "rotate_hour.log"))
	fn := filepath.ToSlash(filepath.Join(dir, "rotate_hour."+time.Now().Add(-1*time.Hour).Format("2006010215")+".log"))
	file, _ := os.Create(fn)
	file.Close()
	fn2 := filepath.ToSlash(filepath.Join(dir, "rotate_hour."+time.Now().Add(-1*time.Hour).Format("2006010215")+".001.log"))
	testFileRotate(t, fn1, fn2, false, true)
}

func TestFileHourlyRotate_04(t *testing.T) {
	dir := t.TempDir()
	fn1 := filepath.ToSlash(filepath.Join(dir, "rotate_hour.log"))
	fn2 := filepath.ToSlash(filepath.Join(dir, "rotate_hour."+time.Now().Add(-1*time.Hour).Format("2006010215")+".001.log"))
	testFileHourlyRotate(t, fn1, fn2)
}

func TestFileHourlyRotate_05(t *testing.T) {
	dir := t.TempDir()
	fn1 := filepath.ToSlash(filepath.Join(dir, "rotate_hour.log"))
	fn := filepath.ToSlash(filepath.Join(dir, "rotate_hour."+time.Now().Add(-1*time.Hour).Format("2006010215")+".log"))
	file, _ := os.Create(fn)
	file.Close()
	fn2 := filepath.ToSlash(filepath.Join(dir, "rotate_hour."+time.Now().Add(-1*time.Hour).Format("2006010215")+".001.log"))
	testFileHourlyRotate(t, fn1, fn2)
}

func TestFileHourlyRotate_06(t *testing.T) { // test file mode
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permission bits are not reliable on Windows")
	}
	log := NewLogger(10000)
	filename := testLogFile(t, "test3.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q, "hourly":true, "maxlines":4}`, filename))
	defer log.Close()
	log.Debug("debug")
	log.Info("info")
	log.Notice("notice")
	log.Warning("warning")
	log.Error("error")
	log.Alert("alert")
	log.Critical("critical")
	log.Emergency("emergency")
	log.Flush()
	rotateName := filepath.Join(filepath.Dir(filename), "test3"+fmt.Sprintf(".%s.%03d", time.Now().Format("2006010215"), 1)+".log")
	s, _ := os.Lstat(rotateName)
	if s.Mode().Perm() != 0o440 {
		t.Fatal("rotate file mode error")
	}
}

func testFileRotate(t *testing.T, fn1, fn2 string, daily, hourly bool) {
	fw := &fileLogWriter{
		Daily:      daily,
		MaxDays:    7,
		Hourly:     hourly,
		MaxHours:   168,
		Rotate:     true,
		Level:      LevelTrace,
		Perm:       "0660",
		DirPerm:    "0770",
		RotatePerm: "0440",
	}
	fw.logFormatter = fw

	if fw.Daily {
		fw.Init(fmt.Sprintf(`{"filename":"%v","maxdays":1}`, fn1))
		fw.dailyOpenTime = time.Now().Add(-24 * time.Hour)
		fw.dailyOpenDate = fw.dailyOpenTime.Day()
	}

	if fw.Hourly {
		fw.Init(fmt.Sprintf(`{"filename":"%v","maxhours":1}`, fn1))
		fw.hourlyOpenTime = time.Now().Add(-1 * time.Hour)
		fw.hourlyOpenDate = fw.hourlyOpenTime.Day()
	}
	lm := &LogMsg{
		Msg:   "Test message",
		Level: LevelDebug,
		When:  time.Now(),
	}

	fw.WriteMsg(lm)
	fw.Flush()

	for _, file := range []string{fn1, fn2} {
		_, err := os.Stat(file)
		if err != nil {
			t.Log(err)
			t.FailNow()
		}
	}
	fw.Destroy()
}

func testFileDailyRotate(t *testing.T, fn1, fn2 string) {
	fw := &fileLogWriter{
		Daily:      true,
		MaxDays:    7,
		Rotate:     true,
		Level:      LevelTrace,
		Perm:       "0660",
		DirPerm:    "0770",
		RotatePerm: "0440",
	}
	fw.logFormatter = fw

	fw.Init(fmt.Sprintf(`{"filename":"%v","maxdays":1}`, fn1))
	fw.dailyOpenTime = time.Now().Add(-24 * time.Hour)
	fw.dailyOpenDate = fw.dailyOpenTime.Day()
	today, _ := time.ParseInLocation("2006-01-02", time.Now().Format("2006-01-02"), fw.dailyOpenTime.Location())
	today = today.Add(-1 * time.Second)
	fw.dailyRotate(today)
	fw.Flush()
	for _, file := range []string{fn1, fn2} {
		_, err := os.Stat(file)
		if err != nil {
			t.FailNow()
		}
		content, err := ioutil.ReadFile(file)
		if err != nil {
			t.FailNow()
		}
		if len(content) > 0 {
			t.FailNow()
		}
	}
	fw.Destroy()
}

func testFileHourlyRotate(t *testing.T, fn1, fn2 string) {
	fw := &fileLogWriter{
		Hourly:     true,
		MaxHours:   168,
		Rotate:     true,
		Level:      LevelTrace,
		Perm:       "0660",
		DirPerm:    "0770",
		RotatePerm: "0440",
	}

	fw.logFormatter = fw
	fw.Init(fmt.Sprintf(`{"filename":"%v","maxhours":1}`, fn1))
	fw.hourlyOpenTime = time.Now().Add(-1 * time.Hour)
	fw.hourlyOpenDate = fw.hourlyOpenTime.Hour()
	hour, _ := time.ParseInLocation("2006010215", time.Now().Format("2006010215"), fw.hourlyOpenTime.Location())
	hour = hour.Add(-1 * time.Second)
	fw.hourlyRotate(hour)
	fw.Flush()
	for _, file := range []string{fn1, fn2} {
		_, err := os.Stat(file)
		if err != nil {
			t.FailNow()
		}
		content, err := ioutil.ReadFile(file)
		if err != nil {
			t.FailNow()
		}
		if len(content) > 0 {
			t.FailNow()
		}
	}
	fw.Destroy()
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func BenchmarkFile(b *testing.B) {
	log := NewLogger(100000)
	filename := filepath.Join(b.TempDir(), "test4.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q}`, filename))
	defer log.Close()
	for i := 0; i < b.N; i++ {
		log.Debug("debug")
	}
}

func BenchmarkFileAsynchronous(b *testing.B) {
	log := NewLogger(100000)
	filename := filepath.Join(b.TempDir(), "test4.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q}`, filename))
	defer log.Close()
	log.Async()
	for i := 0; i < b.N; i++ {
		log.Debug("debug")
	}
}

func BenchmarkFileCallDepth(b *testing.B) {
	log := NewLogger(100000)
	filename := filepath.Join(b.TempDir(), "test4.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q}`, filename))
	defer log.Close()
	log.EnableFuncCallDepth(true)
	log.SetLogFuncCallDepth(2)
	for i := 0; i < b.N; i++ {
		log.Debug("debug")
	}
}

func BenchmarkFileAsynchronousCallDepth(b *testing.B) {
	log := NewLogger(100000)
	filename := filepath.Join(b.TempDir(), "test4.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q}`, filename))
	defer log.Close()
	log.EnableFuncCallDepth(true)
	log.SetLogFuncCallDepth(2)
	log.Async()
	for i := 0; i < b.N; i++ {
		log.Debug("debug")
	}
}

func BenchmarkFileOnGoroutine(b *testing.B) {
	log := NewLogger(100000)
	filename := filepath.Join(b.TempDir(), "test4.log")
	log.SetLogger("file", fmt.Sprintf(`{"filename":%q}`, filename))
	defer log.Close()
	for i := 0; i < b.N; i++ {
		go log.Debug("debug")
	}
}

func TestFileLogWriter_Format(t *testing.T) {
	lg := &LogMsg{
		Level:      LevelDebug,
		Msg:        "Hello, world",
		When:       time.Date(2020, 9, 19, 20, 12, 37, 9, time.UTC),
		FilePath:   "/user/home/main.go",
		LineNumber: 13,
		Prefix:     "Cus",
	}

	fw := newFileWriter().(*fileLogWriter)
	res := fw.Format(lg)
	assert.Equal(t, "2020/09/19 20:12:37.000  [D] Cus Hello, world\n", res)
}
