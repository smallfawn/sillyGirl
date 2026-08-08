package core

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/smallfawn/sillyGirl/core/common"
)

type pluginRoundTripFunc func(*http.Request) (*http.Response, error)

func (f pluginRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestPluginPublisherIdentity(t *testing.T) {
	if got := pluginPublisherDirName(" SmallFawn "); got != "smallfawn" {
		t.Fatalf("publisher dir = %q; want smallfawn", got)
	}
	if got := pluginPublisherDirName(""); got != "local" {
		t.Fatalf("empty publisher dir = %q; want local", got)
	}
	if got := pluginPublisherDirName("sillygirl-plugin"); got != "sillygirl-plugin" {
		t.Fatalf("explicit publisher dir = %q; want sillygirl-plugin", got)
	}
	if got := pluginPublisherDirName("CON"); got != "con-publisher" {
		t.Fatalf("reserved publisher dir = %q; want con-publisher", got)
	}
	if got := pluginIdentity("SmallFawn", "demo.js"); got != "smallfawn/demo" {
		t.Fatalf("plugin identity = %q; want smallfawn/demo", got)
	}
	if nameUuid(pluginIdentity("author-a", "same.js")) == nameUuid(pluginIdentity("author-b", "same.js")) {
		t.Fatal("same-name plugins from different publishers must have distinct UUIDs")
	}
}

func TestCheckedNodeScriptPathRejectsSymlinkEscape(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	root := nodePluginsRoot()
	authorDir := filepath.Join(root, "author")
	if err := os.MkdirAll(authorDir, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.js")
	if err := os.WriteFile(outside, []byte("console.log('outside')"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(authorDir, "escape.js")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	if got, err := checkedNodeScriptPath(link); err == nil {
		t.Fatalf("symlink escape accepted as %q", got)
	}
	for _, entry := range discoverNodePluginScripts(root) {
		if samePath(entry.Path, link) {
			t.Fatalf("symlink escape was discovered as plugin: %#v", entry)
		}
	}
}

func TestDiscoverNodePluginScriptsIncludesPublisherDirectories(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	root := nodePluginsRoot()
	for name, content := range map[string]string{
		"legacy.js":                   "// legacy",
		"smallfawn/normal.js":         "// normal",
		"smallfawn/sharedTools.js":    "// [module:true]",
		"another/normal.js":           "// same filename",
		"smallfawn/nested/ignored.js": "// too deep",
		"node_modules/ignored.js":     "// runtime dependency",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	entries := discoverNodePluginScripts(root)
	identities := make([]string, 0, len(entries))
	for _, entry := range entries {
		identities = append(identities, entry.Identity)
	}
	want := []string{"another/normal", "legacy", "smallfawn/normal", "smallfawn/sharedTools"}
	if !reflect.DeepEqual(identities, want) {
		t.Fatalf("discovered identities = %#v; want %#v", identities, want)
	}
	if got := nodePluginNameIndexByUUID(); got[nameUuid("smallfawn/normal")] != "smallfawn/normal" || got[nameUuid("another/normal")] != "another/normal" {
		t.Fatalf("UUID index does not preserve publisher identity: %#v", got)
	}
	if _, err := pluginScriptPath("normal", NODE); err == nil {
		t.Fatal("ambiguous bare plugin name should require publisher/name")
	}
	path, err := pluginScriptPath("smallfawn/normal", NODE)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(path) != filepath.Join(root, "smallfawn", "normal.js") {
		t.Fatalf("publisher-scoped path = %q", path)
	}
}

func TestModuleDependencyIsScopedToPublisher(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	root := nodePluginsRoot()
	write := func(author, name, source string) string {
		path := filepath.Join(root, author, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	moduleA := &common.Function{UUID: "module-a", Title: "Shared A", Type: NODE, Module: true, Path: write("author-a", "sharedTools.js", "module.exports = {}")}
	moduleB := &common.Function{UUID: "module-b", Title: "Shared B", Type: NODE, Module: true, Path: write("author-b", "sharedTools.js", "module.exports = {}")}
	consumerA := &common.Function{UUID: "consumer-a", Title: "Consumer A", Type: NODE, Path: write("author-a", "consumer.js", `// [depe: ["./sharedTools.js"]]`)}
	consumerB := &common.Function{UUID: "consumer-b", Title: "Consumer B", Type: NODE, Path: write("author-b", "consumer.js", `// [depe: ["./sharedTools.js"]]`)}
	installed := []*common.Function{moduleA, moduleB, consumerA, consumerB}

	resolved, found, err := lookupModuleDependency("./sharedTools.js", NODE, "author-a", installed)
	if err != nil || !found || resolved != moduleA {
		t.Fatalf("author-a module lookup = (%#v, %v, %v)", resolved, found, err)
	}
	dependents, err := pluginModuleDependents(moduleA, installed)
	if err != nil {
		t.Fatal(err)
	}
	if len(dependents) != 1 || dependents[0].ID != "consumer-a" {
		t.Fatalf("author-a dependents = %#v; want only consumer-a", dependents)
	}
}

func TestInstallGithubPluginUsesPublisherDirectoryAndSharedRuntime(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	script := `// [title: Shared Tools]
// [name: sharedTools]
// [desc: shared fixture]
// [version: v1.0.0]
// [module: true]
// [status: true]
// [public: false]

module.exports = {};
`
	previousTransport := http.DefaultTransport
	http.DefaultTransport = pluginRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(script)),
			Request:    req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	source := &githubPluginSource{Owner: "SmallFawn", Repo: "Plugin-Market", Branch: "main"}
	address := makeGithubNodePluginAddress(source, "plugins/sharedTools.js", "", NODE)
	if err := installGithubNodePlugin(address); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pluginLock.Lock()
		unloadNodePluginLocked(nameUuid("smallfawn/sharedTools"))
		pluginLock.Unlock()
	})

	root := nodePluginsRoot()
	pluginPath := filepath.Join(root, "smallfawn", "sharedTools.js")
	if data, err := os.ReadFile(pluginPath); err != nil || string(data) != script {
		t.Fatalf("publisher plugin = (%q, %v); want downloaded source", string(data), err)
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		t.Fatalf("shared package.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "node_modules", "sillygirl", "index.js")); err != nil {
		t.Fatalf("shared sillygirl runtime missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "smallfawn", "package.json")); !os.IsNotExist(err) {
		t.Fatalf("publisher directory unexpectedly owns package.json: %v", err)
	}
	installed, err := nodeFunctionByID(nameUuid("smallfawn/sharedTools"))
	if err != nil || !installed.Module || !samePath(installed.Path, pluginPath) {
		t.Fatalf("installed publisher plugin = (%#v, %v)", installed, err)
	}

}

func TestInstallGithubPluginRejectsPublisherSymlinkEscape(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	root := nodePluginsRoot()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	publisherLink := filepath.Join(root, "smallfawn")
	if err := os.Symlink(outside, publisherLink); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	previousTransport := http.DefaultTransport
	http.DefaultTransport = pluginRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("// [title: Escape]\n// [module: true]\n")),
			Request:    req,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	source := &githubPluginSource{Owner: "SmallFawn", Repo: "Plugin-Market", Branch: "main"}
	address := makeGithubNodePluginAddress(source, "plugins/escape.js", "", NODE)
	if err := installGithubNodePlugin(address); err == nil {
		t.Fatal("publisher symlink escape was accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape.js")); !os.IsNotExist(err) {
		t.Fatalf("installer wrote outside plugin root: %v", err)
	}
}
