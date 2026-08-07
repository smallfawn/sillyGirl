package core

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/utils"
)

const (
	maxReleaseMetadataBytes  int64 = 8 << 20
	maxReleaseArchiveBytes   int64 = 512 << 20
	maxReleaseExtractedBytes int64 = 1 << 30
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releasePayload struct {
	TagName string         `json:"tag_name"`
	Name    string         `json:"name"`
	HTMLURL string         `json:"html_url"`
	Assets  []releaseAsset `json:"assets"`
}

type systemUpdateResult struct {
	Mode    string `json:"mode"`
	Repo    string `json:"repo"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Changed bool   `json:"changed"`
	Asset   string `json:"asset"`
	Output  string `json:"output"`
}

type systemUpdateSnapshot struct {
	ID        string              `json:"id"`
	Running   bool                `json:"running"`
	Status    string              `json:"status"`
	Percent   int                 `json:"percent"`
	Message   string              `json:"message"`
	Error     string              `json:"error"`
	Result    *systemUpdateResult `json:"result"`
	StartedAt int64               `json:"started_at"`
	UpdatedAt int64               `json:"updated_at"`
}

type systemRestartJob struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

var systemRestartState = struct {
	sync.Mutex
	Job systemRestartJob
}{}

var systemUpdateState = struct {
	sync.Mutex
	systemUpdateSnapshot
}{
	systemUpdateSnapshot: systemUpdateSnapshot{
		Status:  "idle",
		Percent: 0,
		Message: "未开始",
	},
}

func init() {
	GinApi(GET, "/api/health", func(ctx *gin.Context) {
		ApiOK(ctx, map[string]interface{}{
			"status":     "ok",
			"version":    currentAppVersion(),
			"started_at": sillyGirl.GetString("started_at"),
		})
	})
	GinApi(POST, "/api/admin/system-update-jobs", RequireAuth, func(ctx *gin.Context) {
		snapshot, created := startSystemUpdateJob()
		if !created {
			ApiConflict(ctx, "已有系统更新任务正在运行")
			return
		}
		ApiAccepted(ctx, "/api/admin/system-update-jobs/"+snapshot.ID, snapshot)
	})
	GinApi(GET, "/api/admin/system-update-jobs/:id", RequireAuth, func(ctx *gin.Context) {
		snapshot := systemUpdateSnapshotCopy()
		if snapshot.ID == "" || snapshot.ID != strings.TrimSpace(ctx.Param("id")) {
			ApiNotFound(ctx, "系统更新任务不存在")
			return
		}
		ApiOK(ctx, snapshot)
	})
	GinApi(POST, "/api/admin/system-restart-jobs", RequireAuth, func(ctx *gin.Context) {
		job := systemRestartJob{ID: utils.GenUUID(), Status: "accepted", CreatedAt: time.Now().Unix()}
		systemRestartState.Lock()
		systemRestartState.Job = job
		systemRestartState.Unlock()
		ApiAccepted(ctx, "/api/admin/system-restart-jobs/"+job.ID, job)
		go func() {
			time.Sleep(time.Second)
			if runtime.GOOS == "windows" && readyExecutablePath() != "" {
				utils.Daemon("ready")
				return
			}
			utils.Daemon()
		}()
	})
	GinApi(GET, "/api/admin/system-restart-jobs/:id", RequireAuth, func(ctx *gin.Context) {
		systemRestartState.Lock()
		job := systemRestartState.Job
		systemRestartState.Unlock()
		if job.ID == "" || job.ID != strings.TrimSpace(ctx.Param("id")) {
			ApiNotFound(ctx, "系统重启任务不存在")
			return
		}
		ApiOK(ctx, job)
	})
}

func startSystemUpdateJob() (systemUpdateSnapshot, bool) {
	systemUpdateState.Lock()
	if systemUpdateState.Running {
		snapshot := systemUpdateState.systemUpdateSnapshot
		systemUpdateState.Unlock()
		return snapshot, false
	}
	now := time.Now().Unix()
	systemUpdateState.systemUpdateSnapshot = systemUpdateSnapshot{
		ID:        utils.GenUUID(),
		Running:   true,
		Status:    "running",
		Percent:   3,
		Message:   "更新任务已启动",
		StartedAt: now,
		UpdatedAt: now,
	}
	snapshot := systemUpdateState.systemUpdateSnapshot
	systemUpdateState.Unlock()

	go func() {
		result, err := updateFromRelease(setSystemUpdateProgress)
		systemUpdateState.Lock()
		defer systemUpdateState.Unlock()
		systemUpdateState.Running = false
		systemUpdateState.UpdatedAt = time.Now().Unix()
		if err != nil {
			systemUpdateState.Status = "error"
			systemUpdateState.Error = err.Error()
			systemUpdateState.Message = err.Error()
			return
		}
		systemUpdateState.Status = "done"
		systemUpdateState.Percent = 100
		systemUpdateState.Message = "更新完成，请选择是否立即重启"
		systemUpdateState.Result = result
	}()
	return snapshot, true
}

func setSystemUpdateProgress(percent int, message string) {
	systemUpdateState.Lock()
	defer systemUpdateState.Unlock()
	if percent > systemUpdateState.Percent {
		systemUpdateState.Percent = percent
	}
	systemUpdateState.Message = message
	systemUpdateState.UpdatedAt = time.Now().Unix()
}

func systemUpdateSnapshotCopy() systemUpdateSnapshot {
	systemUpdateState.Lock()
	defer systemUpdateState.Unlock()
	return systemUpdateState.systemUpdateSnapshot
}

func updateFromRelease(progress func(int, string)) (*systemUpdateResult, error) {
	progress(8, "正在读取 GitHub Release")
	before := currentAppVersion()
	release, err := fetchLatestRelease()
	if err != nil {
		return nil, err
	}
	progress(22, "正在选择当前系统的 Release 包")
	asset := selectReleaseAsset(release)
	if asset.Name == "" || asset.BrowserDownloadURL == "" {
		return nil, fmt.Errorf("未找到适配当前系统的 Release 包：%s", releaseName(release))
	}
	tmpDir, err := os.MkdirTemp("", "sillygirl-update-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, safeReleaseFileName(asset.Name))
	progress(32, "正在下载 Release 包："+asset.Name)
	if err := downloadReleaseFile(asset.BrowserDownloadURL, archivePath); err != nil {
		return nil, err
	}
	progress(68, "正在校验 Release 包")
	if err := verifyReleaseChecksum(release, asset, archivePath); err != nil {
		return nil, err
	}
	progress(78, "正在解压 Release 包")
	if err := extractReleaseArchive(archivePath, tmpDir); err != nil {
		return nil, err
	}
	progress(88, "正在安装 Release 文件")
	if err := installReleasePayload(tmpDir); err != nil {
		return nil, err
	}
	after := normalizeAppVersion(release.TagName)
	if after == "" {
		after = normalizeAppVersion(release.Name)
	}
	rememberLatestAppVersion(after, release.HTMLURL)
	return &systemUpdateResult{
		Mode:    "release",
		Repo:    release.HTMLURL,
		Before:  before,
		After:   after,
		Changed: before != after || after == "",
		Asset:   asset.Name,
		Output:  fmt.Sprintf("已下载并安装 Release 包：%s", asset.Name),
	}, nil
}

func fetchLatestRelease() (*releasePayload, error) {
	address := "https://api.github.com/repos/smallfawn/sillyGirl/releases/latest"
	var lastErr error
	for _, url := range releaseURLs(address) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "sillyGirl")
		resp, err := releaseHTTPClient(45 * time.Second).Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxReleaseMetadataBytes+1))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s HTTP %d", url, resp.StatusCode)
			continue
		}
		if int64(len(data)) > maxReleaseMetadataBytes {
			lastErr = fmt.Errorf("GitHub Release 响应超过 %d MiB 限制", maxReleaseMetadataBytes>>20)
			continue
		}
		release := &releasePayload{}
		if err := json.Unmarshal(data, release); err != nil {
			lastErr = fmt.Errorf("GitHub Release 接口返回非 JSON：%s", string(data[:min(len(data), 200)]))
			continue
		}
		return release, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("GitHub Release 请求失败")
	}
	return nil, lastErr
}

func selectReleaseAsset(release *releasePayload) releaseAsset {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goarch == "386" {
		goarch = "386"
	}
	suffix := ".tar.gz"
	if goos == "windows" {
		suffix = ".zip"
	}
	for _, asset := range release.Assets {
		name := asset.Name
		if strings.Contains(name, "_"+goos+"_"+goarch) && strings.HasSuffix(name, suffix) {
			return asset
		}
	}
	return releaseAsset{}
}

func verifyReleaseChecksum(release *releasePayload, asset releaseAsset, archivePath string) error {
	var checksumAsset releaseAsset
	for _, item := range release.Assets {
		if strings.EqualFold(item.Name, "checksums.txt") {
			checksumAsset = item
			break
		}
	}
	if checksumAsset.BrowserDownloadURL == "" {
		return fmt.Errorf("Release 缺少 checksums.txt，拒绝更新")
	}
	checksumPath := archivePath + ".checksums.txt"
	if err := downloadReleaseFile(checksumAsset.BrowserDownloadURL, checksumPath); err != nil {
		return err
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}
	expected := parseReleaseChecksum(string(data), asset.Name)
	if expected == "" {
		return fmt.Errorf("checksums.txt 中缺少 %s 的 SHA256", asset.Name)
	}
	actual, err := sha256File(archivePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("Release 包校验失败：%s", asset.Name)
	}
	return nil
}

func parseReleaseChecksum(text string, fileName string) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		checkedName := filepath.Base(strings.TrimPrefix(parts[1], "*"))
		if checkedName == fileName && len(parts[0]) == 64 {
			return parts[0]
		}
	}
	return ""
}

func downloadReleaseFile(address string, target string) error {
	var lastErr error
	for _, url := range releaseURLs(address) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "sillyGirl")
		resp, err := releaseHTTPClient(180 * time.Second).Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s HTTP %d", url, resp.StatusCode)
			resp.Body.Close()
			continue
		}
		if resp.ContentLength > maxReleaseArchiveBytes {
			lastErr = fmt.Errorf("Release 包超过 %d MiB 限制", maxReleaseArchiveBytes>>20)
			resp.Body.Close()
			continue
		}
		file, err := os.Create(target)
		if err != nil {
			resp.Body.Close()
			return err
		}
		_, copyErr := copyWithLimit(file, resp.Body, maxReleaseArchiveBytes)
		closeErr := file.Close()
		resp.Body.Close()
		if copyErr != nil {
			_ = os.Remove(target)
			lastErr = copyErr
			continue
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("下载失败：%s", address)
	}
	return lastErr
}

func extractReleaseArchive(archivePath string, targetDir string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZipArchive(archivePath, targetDir)
	}
	return extractTarGzArchive(archivePath, targetDir)
}

func extractTarGzArchive(archivePath string, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var extracted int64
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		dest, err := safeExtractPath(targetDir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if header.Size < 0 || header.Size > maxReleaseExtractedBytes-extracted {
				return fmt.Errorf("Release 解压内容超过 %d MiB 限制", maxReleaseExtractedBytes>>20)
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)&0777)
			if err != nil {
				return err
			}
			written, copyErr := io.CopyN(out, tr, header.Size)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			extracted += written
		}
	}
}

func extractZipArchive(archivePath string, targetDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	var extracted int64
	for _, file := range reader.File {
		dest, err := safeExtractPath(targetDir, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return err
			}
			continue
		}
		remaining := maxReleaseExtractedBytes - extracted
		if file.UncompressedSize64 > uint64(remaining) {
			return fmt.Errorf("Release 解压内容超过 %d MiB 限制", maxReleaseExtractedBytes>>20)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			src.Close()
			return err
		}
		written, copyErr := copyWithLimit(out, src, remaining)
		closeErr := out.Close()
		src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		extracted += written
	}
	return nil
}

func copyWithLimit(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	if limit < 0 {
		return 0, errors.New("写入大小限制无效")
	}
	written, err := io.CopyN(dst, src, limit+1)
	if written > limit {
		return written, fmt.Errorf("内容超过 %d MiB 限制", limit>>20)
	}
	if errors.Is(err, io.EOF) {
		return written, nil
	}
	return written, err
}

func installReleasePayload(tmpDir string) error {
	binary, err := findReleaseBinary(tmpDir)
	if err != nil {
		return err
	}
	executablePath, err := releaseExecutablePath()
	if err != nil {
		return err
	}
	targetDir := filepath.Dir(executablePath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return updateCopyFile(binary, readyExecutablePath())
	}
	tmpTarget := fmt.Sprintf("%s.new-%d", executablePath, time.Now().UnixNano())
	backup := fmt.Sprintf("%s.bak-%d", executablePath, time.Now().UnixNano())
	if err := updateCopyFile(binary, tmpTarget); err != nil {
		return err
	}
	if err := os.Chmod(tmpTarget, 0755); err != nil {
		_ = os.Remove(tmpTarget)
		return err
	}
	backedUp := false
	if _, err := os.Stat(executablePath); err == nil {
		if err := os.Rename(executablePath, backup); err != nil {
			_ = os.Remove(tmpTarget)
			return err
		}
		backedUp = true
	}
	if err := os.Rename(tmpTarget, executablePath); err != nil {
		_ = os.Remove(tmpTarget)
		if backedUp {
			_ = os.Rename(backup, executablePath)
		}
		return err
	}
	_ = os.Remove(backup)
	if proto3, err := findReleaseProto3(tmpDir); err == nil && proto3 != "" {
		if err := updateCopyDir(proto3, filepath.Join(targetDir, "proto3")); err != nil {
			return err
		}
	}
	return nil
}

func releaseExecutablePath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("SILLYGIRL_EXEC_PATH")); configured != "" {
		return filepath.Abs(configured)
	}
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func readyExecutablePath() string {
	path, err := releaseExecutablePath()
	if err != nil || !strings.HasSuffix(strings.ToLower(path), ".exe") {
		return ""
	}
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".ready.exe"
}

func findReleaseBinary(root string) (string, error) {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || found != "" {
			return err
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "sillyGirl_") {
			return nil
		}
		if suffix != "" && !strings.HasSuffix(strings.ToLower(name), suffix) {
			return nil
		}
		if suffix == "" && (strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tar.gz")) {
			return nil
		}
		found = path
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("Release 包中没有找到 sillyGirl 可执行文件")
	}
	return found, nil
}

func findReleaseProto3(root string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || found != "" {
			return err
		}
		if entry.Name() == "proto3" {
			if _, statErr := os.Stat(filepath.Join(path, "sillygirl.js")); statErr == nil {
				found = path
			}
		}
		return nil
	})
	return found, err
}

func updateCopyDir(source string, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || rel == "." {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		return updateCopyFile(path, dest)
	})
}

func updateCopyFile(source string, target string) error {
	if target == "" {
		return fmt.Errorf("目标文件路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func safeExtractPath(root string, name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return "", fmt.Errorf("Release 包包含非法路径：%s", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("Release 包包含非法路径：%s", name)
	}
	dest := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, dest)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("Release 包包含非法路径：%s", name)
	}
	return dest, nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func releaseURLs(address string) []string {
	urls := []string{}
	for _, prefix := range releaseGithubProxyPrefixes() {
		urls = append(urls, strings.TrimRight(prefix, "/")+"/"+address)
	}
	urls = append(urls, address)
	return uniqueStrings(urls)
}

func releaseHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: 8 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
}

func releaseGithubProxyPrefixes() []string {
	values := []string{}
	if selected := strings.TrimSpace(githubAcceleratorPrefix()); selected != "" {
		values = append(values, selected)
	}
	for _, prefix := range builtinGithubAccelerators {
		if strings.TrimSpace(prefix) != "" {
			values = append(values, strings.TrimSpace(prefix))
		}
	}
	if len(values) == 0 {
		values = append(values, "https://gh-proxy.org")
	}
	return uniqueStrings(values)
}

func uniqueStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func safeReleaseFileName(name string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(strings.TrimSpace(name))
}

func releaseName(release *releasePayload) string {
	if release == nil {
		return "latest"
	}
	if release.TagName != "" {
		return release.TagName
	}
	if release.Name != "" {
		return release.Name
	}
	return "latest"
}
