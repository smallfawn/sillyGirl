package core

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

const systemBackupFormat = "sillygirl-backup-v1"

type systemBackupManifest struct {
	Format         string   `json:"format"`
	AppVersion     string   `json:"app_version"`
	CreatedAt      string   `json:"created_at"`
	StorageBackend string   `json:"storage_backend"`
	BucketCount    int      `json:"bucket_count"`
	KeyCount       int      `json:"key_count"`
	FileCount      int      `json:"file_count"`
	FileBytes      int64    `json:"file_bytes"`
	Excluded       []string `json:"excluded"`
}

type systemBackupStorage struct {
	Format  string               `json:"format"`
	Buckets []systemBackupBucket `json:"buckets"`
}

type systemBackupBucket struct {
	Name    string              `json:"name"`
	Entries []systemBackupEntry `json:"entries"`
}

type systemBackupEntry struct {
	KeyBase64   string `json:"key_base64"`
	ValueBase64 string `json:"value_base64"`
}

func init() {
	GinApi(GET, "/api/admin/backup", RequireAuth, downloadSystemBackup)
}

func downloadSystemBackup(ctx *gin.Context) {
	file, err := os.CreateTemp("", "sillygirl-backup-*.zip")
	if err != nil {
		ApiError(ctx, http.StatusInternalServerError, "创建备份文件失败："+err.Error())
		return
	}
	path := file.Name()
	defer os.Remove(path)

	createdAt := time.Now()
	if _, err := writeSystemBackup(file, MakeBucket(""), utils.GetDataHome(), createdAt); err != nil {
		_ = file.Close()
		ApiError(ctx, http.StatusInternalServerError, "生成备份失败："+err.Error())
		return
	}
	if err := file.Close(); err != nil {
		ApiError(ctx, http.StatusInternalServerError, "写入备份失败："+err.Error())
		return
	}

	ctx.Header("Cache-Control", "no-store")
	ctx.FileAttachment(path, fmt.Sprintf("sillygirl-backup-%s.zip", createdAt.Format("20060102-150405")))
}

func writeSystemBackup(dst io.Writer, root storage.Bucket, dataHome string, createdAt time.Time) (systemBackupManifest, error) {
	if dst == nil {
		return systemBackupManifest{}, errors.New("备份输出为空")
	}
	if root == nil {
		return systemBackupManifest{}, errors.New("存储后端未初始化")
	}

	writer := zip.NewWriter(dst)
	storageSnapshot := snapshotSystemBackupStorage(root)
	storageData, err := json.MarshalIndent(storageSnapshot, "", "  ")
	if err != nil {
		_ = writer.Close()
		return systemBackupManifest{}, err
	}
	if err := writeSystemBackupBytes(writer, "storage.json", storageData, createdAt); err != nil {
		_ = writer.Close()
		return systemBackupManifest{}, err
	}

	fileCount, fileBytes, err := writeSystemBackupFiles(writer, dataHome)
	if err != nil {
		_ = writer.Close()
		return systemBackupManifest{}, err
	}
	keyCount := 0
	for _, bucket := range storageSnapshot.Buckets {
		keyCount += len(bucket.Entries)
	}
	manifest := systemBackupManifest{
		Format:         systemBackupFormat,
		AppVersion:     currentAppVersion(),
		CreatedAt:      createdAt.Format(time.RFC3339),
		StorageBackend: root.Type(),
		BucketCount:    len(storageSnapshot.Buckets),
		KeyCount:       keyCount,
		FileCount:      fileCount,
		FileBytes:      fileBytes,
		Excluded: []string{
			"sillyGirl.db and database lock files",
			"PID files",
			"node_modules and package caches",
			"Python bytecode caches",
			"temporary and backup directories",
		},
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = writer.Close()
		return systemBackupManifest{}, err
	}
	if err := writeSystemBackupBytes(writer, "manifest.json", manifestData, createdAt); err != nil {
		_ = writer.Close()
		return systemBackupManifest{}, err
	}
	if err := writer.Close(); err != nil {
		return systemBackupManifest{}, err
	}
	return manifest, nil
}

func snapshotSystemBackupStorage(root storage.Bucket) systemBackupStorage {
	names := append([]string(nil), root.Buckets()...)
	sort.Strings(names)
	snapshot := systemBackupStorage{
		Format:  systemBackupFormat,
		Buckets: make([]systemBackupBucket, 0, len(names)),
	}
	for _, name := range names {
		bucket := systemBackupBucket{Name: name, Entries: []systemBackupEntry{}}
		root.Copy(name).Foreach(func(key, value []byte) error {
			bucket.Entries = append(bucket.Entries, systemBackupEntry{
				KeyBase64:   base64.StdEncoding.EncodeToString(key),
				ValueBase64: base64.StdEncoding.EncodeToString(value),
			})
			return nil
		})
		sort.Slice(bucket.Entries, func(i, j int) bool {
			return bucket.Entries[i].KeyBase64 < bucket.Entries[j].KeyBase64
		})
		snapshot.Buckets = append(snapshot.Buckets, bucket)
	}
	return snapshot
}

func writeSystemBackupFiles(writer *zip.Writer, dataHome string) (int, int64, error) {
	dataHome = strings.TrimSpace(dataHome)
	if dataHome == "" {
		return 0, 0, nil
	}
	info, err := os.Stat(dataHome)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if !info.IsDir() {
		return 0, 0, errors.New("数据目录不是文件夹")
	}

	fileCount := 0
	var fileBytes int64
	err = filepath.Walk(dataHome, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dataHome, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldExcludeSystemBackupPath(rel, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		cleanRel := filepath.ToSlash(filepath.Clean(rel))
		if cleanRel == ".." || strings.HasPrefix(cleanRel, "../") {
			return errors.New("备份文件路径越界")
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = "files/" + cleanRel
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(entry, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		fileCount++
		fileBytes += written
		return nil
	})
	return fileCount, fileBytes, err
}

func shouldExcludeSystemBackupPath(rel string, isDir bool) bool {
	clean := strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
	base := strings.ToLower(filepath.Base(clean))
	if isDir {
		switch base {
		case ".git", ".pnpm-store", "node_modules", "__pycache__", "backups", "cache", "temp", "tmp":
			return true
		}
		return false
	}
	if !strings.Contains(clean, "/") {
		switch base {
		case "sillygirl.db", "sillygirl.db.lock", "sillygirl.pid":
			return true
		}
	}
	for _, suffix := range []string{".pyc", ".pyo", ".lock", ".bak", ".tmp", ".temp", ".log"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return strings.HasSuffix(base, "~")
}

func writeSystemBackupBytes(writer *zip.Writer, name string, data []byte, modifiedAt time.Time) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetModTime(modifiedAt)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}
