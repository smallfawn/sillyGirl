package core

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/smallfawn/sillyGirl/core/storage"
)

type backupTestStore struct {
	buckets map[string]map[string][]byte
}

type backupTestBucket struct {
	store *backupTestStore
	name  string
}

func (b *backupTestBucket) Set(interface{}, interface{}) (string, bool, error) {
	return "", false, nil
}
func (b *backupTestBucket) Set2(interface{}, interface{}) (string, bool, error) {
	return "", false, nil
}
func (b *backupTestBucket) Copy(name string) storage.Bucket {
	return &backupTestBucket{store: b.store, name: name}
}
func (b *backupTestBucket) IsEmpty() (bool, error) { return len(b.store.buckets[b.name]) == 0, nil }
func (b *backupTestBucket) Size() (int64, error)   { return int64(len(b.store.buckets[b.name])), nil }
func (b *backupTestBucket) Delete() error          { return nil }
func (b *backupTestBucket) Type() string           { return "fixture" }
func (b *backupTestBucket) Buckets() []string {
	result := make([]string, 0, len(b.store.buckets))
	for name := range b.store.buckets {
		result = append(result, name)
	}
	return result
}
func (b *backupTestBucket) GetString(values ...interface{}) string { return "" }
func (b *backupTestBucket) GetBytes(key string) []byte             { return b.store.buckets[b.name][key] }
func (b *backupTestBucket) GetInt(string, ...int) int              { return 0 }
func (b *backupTestBucket) GetBool(string, ...bool) bool           { return false }
func (b *backupTestBucket) Foreach(handle func([]byte, []byte) error) {
	for key, value := range b.store.buckets[b.name] {
		_ = handle([]byte(key), value)
	}
}
func (b *backupTestBucket) Create(interface{}) error { return nil }
func (b *backupTestBucket) First(interface{}) error  { return nil }
func (b *backupTestBucket) String() string           { return b.name }
func (b *backupTestBucket) GetName() string          { return b.name }
func (b *backupTestBucket) Keys() ([]string, error)  { return nil, nil }

func TestWriteSystemBackupIncludesStorageAndSourceFiles(t *testing.T) {
	dataHome := t.TempDir()
	writeBackupTestFile(t, dataHome, "plugins/demo.js", "console.log('demo')")
	writeBackupTestFile(t, dataHome, "plugins/node_modules/pkg/index.js", "excluded")
	writeBackupTestFile(t, dataHome, "plugins/demo.js.bak", "excluded")
	writeBackupTestFile(t, dataHome, "runtime/__pycache__/demo.pyc", "excluded")
	writeBackupTestFile(t, dataHome, "sillyGirl.db", "excluded")
	writeBackupTestFile(t, dataHome, "sillyGirl.pid", "excluded")
	writeBackupTestFile(t, dataHome, "profile.json", `{"ok":true}`)

	root := &backupTestBucket{store: &backupTestStore{buckets: map[string]map[string][]byte{
		"zeta":  {"token": []byte{0, 1, 2, 255}},
		"alpha": {"name": []byte("SillyGirl")},
	}}}
	createdAt := time.Date(2026, 8, 4, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	var output bytes.Buffer
	manifest, err := writeSystemBackup(&output, root, dataHome, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.StorageBackend != "fixture" || manifest.BucketCount != 2 || manifest.KeyCount != 2 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if manifest.FileCount != 2 {
		t.Fatalf("file count = %d, want 2", manifest.FileCount)
	}

	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}
	for _, name := range []string{"manifest.json", "storage.json", "files/plugins/demo.js", "files/profile.json"} {
		if files[name] == nil {
			t.Fatalf("backup missing %s", name)
		}
	}
	for _, name := range []string{"files/sillyGirl.db", "files/sillyGirl.pid", "files/plugins/demo.js.bak", "files/plugins/node_modules/pkg/index.js", "files/runtime/__pycache__/demo.pyc"} {
		if files[name] != nil {
			t.Fatalf("backup unexpectedly contains %s", name)
		}
	}

	storageData := readBackupZipFile(t, files["storage.json"])
	var snapshot systemBackupStorage
	if err := json.Unmarshal(storageData, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Buckets) != 2 || snapshot.Buckets[0].Name != "alpha" || snapshot.Buckets[1].Name != "zeta" {
		t.Fatalf("buckets are not stable and sorted: %+v", snapshot.Buckets)
	}
	wantValue := base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 255})
	if snapshot.Buckets[1].Entries[0].ValueBase64 != wantValue {
		t.Fatalf("binary value = %q, want %q", snapshot.Buckets[1].Entries[0].ValueBase64, wantValue)
	}
}

func TestShouldExcludeSystemBackupPath(t *testing.T) {
	tests := []struct {
		path string
		dir  bool
		want bool
	}{
		{"sillyGirl.db", false, true},
		{"sillyGirl.pid", false, true},
		{"plugins/demo.js", false, false},
		{"plugins/demo.js.bak", false, true},
		{"plugins/node_modules", true, true},
		{"runtime/__pycache__", true, true},
		{"plugins/dependencies.lock", false, true},
		{"profile.json", false, false},
	}
	for _, test := range tests {
		if got := shouldExcludeSystemBackupPath(test.path, test.dir); got != test.want {
			t.Fatalf("shouldExcludeSystemBackupPath(%q, %v) = %v, want %v", test.path, test.dir, got, test.want)
		}
	}
}

func writeBackupTestFile(t *testing.T, root string, name string, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readBackupZipFile(t *testing.T, file *zip.File) []byte {
	t.Helper()
	reader, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
