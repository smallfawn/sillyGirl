package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckMainIndexPython(t *testing.T) {
	class, ok := CheckMainIndex("hello.py")
	if !ok || class != PYTHON {
		t.Fatalf("CheckMainIndex(hello.py) = (%q, %v); want (%q, true)", class, ok, PYTHON)
	}
}

func TestFindMainIndexPython(t *testing.T) {
	dir := t.TempDir()
	flat := filepath.Join(dir, "flat.py")
	if err := os.WriteFile(flat, []byte(`print("ok")`), 0644); err != nil {
		t.Fatal(err)
	}
	if got, class := FindMainIndex(flat); filepath.Clean(got) != filepath.Clean(flat) || class != PYTHON {
		t.Fatalf("FindMainIndex(flat.py) = (%q, %q); want (%q, %q)", got, class, flat, PYTHON)
	}

	mainDir := filepath.Join(dir, "plugin")
	if err := os.Mkdir(mainDir, 0755); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(mainDir, "main.py")
	if err := os.WriteFile(main, []byte(`print("ok")`), 0644); err != nil {
		t.Fatal(err)
	}
	if got, class := FindMainIndex(mainDir); filepath.Clean(got) != filepath.Clean(main) || class != PYTHON {
		t.Fatalf("FindMainIndex(main.py) = (%q, %q); want (%q, %q)", got, class, main, PYTHON)
	}
}

func TestCreatePythonPluginWritesFlatScriptAndRuntime(t *testing.T) {
	t.Setenv("SILLYGIRL_DATA_PATH", t.TempDir())
	fileName, err := normalizeNodeScriptFileName("python-smoke.py")
	if err != nil {
		t.Fatal(err)
	}
	root, index, err := createNodePlugin("python-smoke", "PythonSmoke", fileName, PYTHON)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(root) != nodePluginsRoot() {
		t.Fatalf("createNodePlugin root = %q; want %q", root, nodePluginsRoot())
	}
	if filepath.Base(index) != "python-smoke.py" {
		t.Fatalf("createNodePlugin index = %q; want python-smoke.py", index)
	}
	content, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{"[title: PythonSmoke]", "from sillygirl import sender as s", "asyncio.run(main())"} {
		if !strings.Contains(text, want) {
			t.Fatalf("created python script missing %q:\n%s", want, text)
		}
	}
	if got, class := FindMainIndex(index); filepath.Clean(got) != filepath.Clean(index) || class != PYTHON {
		t.Fatalf("FindMainIndex(created python plugin) = (%q, %q); want (%q, %q)", got, class, index, PYTHON)
	}
	runtimeDir, err := ensurePythonSillygirlModule()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sillygirl.py", "srpc_pb2.py", "srpc_pb2_grpc.py"} {
		if _, err := os.Stat(filepath.Join(runtimeDir, name)); err != nil {
			t.Fatalf("missing python runtime file %s: %v", name, err)
		}
	}
}
