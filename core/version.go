package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	appVersion          = "0.0.1"
	appRepository       = "https://github.com/smallfawn/sillyGirl"
	remoteVersionRawURL = "https://raw.githubusercontent.com/smallfawn/sillyGirl/refs/heads/main/VERSION"
)

var appVersionState = struct {
	sync.RWMutex
	latest    string
	source    string
	checkedAt time.Time
}{
	latest: appVersion,
}

var appVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+([-.+][0-9A-Za-z.-]+)?$`)

func currentAppVersion() string {
	version := normalizeAppVersion(compiled_at)
	if version == "" {
		return appVersion
	}
	return version
}

func latestAppVersion() (string, string) {
	appVersionState.RLock()
	latest := appVersionState.latest
	source := appVersionState.source
	appVersionState.RUnlock()

	latest = normalizeAppVersion(firstNonEmpty(latest, sillyGirl.GetString("remote_version"), sillyGirl.GetString("latest_version")))
	if latest == "" {
		latest = currentAppVersion()
	}
	if source == "" {
		source = versionAcceleratedURLs(remoteVersionRawURL)[0]
	}
	return latest, source
}

func GetVersion() (string, error) {
	latest, source, err := fetchRemoteAppVersion()
	if latest == "" {
		return latestAppVersionFallback(), err
	}
	rememberLatestAppVersion(latest, source)
	if latest != currentAppVersion() {
		console.Log("发现远程版本 %s，当前版本 %s", latest, currentAppVersion())
	}
	return latest, err
}

func refreshAppVersionLoop() {
	for {
		if _, err := GetVersion(); err != nil {
			console.Debug("远程版本检查失败：%v", err)
		}
		time.Sleep(5 * time.Minute)
	}
}

func latestAppVersionFallback() string {
	latest, _ := latestAppVersion()
	if latest == "" {
		return currentAppVersion()
	}
	return latest
}

func fetchRemoteAppVersion() (string, string, error) {
	var lastErr error
	for _, address := range versionAcceleratedURLs(remoteVersionRawURL) {
		req, err := http.NewRequest(http.MethodGet, address, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "sillyGirl")
		resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s HTTP %d", address, resp.StatusCode)
			continue
		}
		fields := strings.Fields(string(data))
		if len(fields) == 0 {
			lastErr = fmt.Errorf("%s 版本内容为空", address)
			continue
		}
		latest := normalizeAppVersion(fields[0])
		if !appVersionPattern.MatchString(latest) {
			lastErr = fmt.Errorf("%s 版本内容无效", address)
			continue
		}
		return latest, address, nil
	}
	if lastErr == nil {
		lastErr = errors.New("没有可用的 GitHub 加速地址")
	}
	return "", "", lastErr
}

func rememberLatestAppVersion(latest string, source string) {
	latest = normalizeAppVersion(latest)
	if latest == "" {
		return
	}
	appVersionState.Lock()
	appVersionState.latest = latest
	appVersionState.source = source
	appVersionState.checkedAt = time.Now()
	appVersionState.Unlock()
	sillyGirl.Set("remote_version", latest)
	sillyGirl.Set("latest_version", latest)
}

func versionAcceleratedURLs(address string) []string {
	prefixes := []string{}
	if selected := strings.TrimSpace(githubAcceleratorPrefix()); selected != "" {
		prefixes = append(prefixes, selected)
	}
	for _, prefix := range builtinGithubAccelerators {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && !Contains(prefixes, prefix) {
			prefixes = append(prefixes, prefix)
		}
	}
	if len(prefixes) == 0 {
		prefixes = append(prefixes, "https://gh-proxy.org")
	}
	urls := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		urls = append(urls, strings.TrimRight(prefix, "/")+"/"+address)
	}
	return urls
}

func normalizeAppVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "refs/tags/")
	version = strings.TrimPrefix(version, "V")
	version = strings.TrimPrefix(version, "v")
	return strings.TrimSpace(version)
}
