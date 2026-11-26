package gate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeKubeconform puts a shell script named kubeconform first on PATH. Its
// first invocation hangs (a stalled schema download), every later one
// answers with one valid resource. Each invocation appends its arguments to
// calls.log in dir.
func fakeKubeconform(t *testing.T) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "$@" >> "` + logPath + `"
cat >/dev/null
if [ ! -f "` + filepath.Join(dir, "hung") + `" ]; then
  touch "` + filepath.Join(dir, "hung") + `"
  exec sleep 30
fi
echo '{"resources":[{"kind":"ConfigMap","name":"settings","status":"statusValid","msg":""}]}'
`
	if err := os.WriteFile(filepath.Join(dir, "kubeconform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func shortenKubeconform(t *testing.T) {
	t.Helper()
	prevTimeout, prevDir := kubeconformAttemptTimeout, kubeconformCacheDir
	kubeconformAttemptTimeout = 2 * time.Second
	kubeconformCacheDir = filepath.Join(t.TempDir(), "cache")
	t.Cleanup(func() { kubeconformAttemptTimeout, kubeconformCacheDir = prevTimeout, prevDir })
}

// A stalled download used to fail the whole gate run. Now one stalled
// process is retried on a fresh one, and the second answer is the verdict.
func TestKubeconformStallIsRetriedOnce(t *testing.T) {
	logPath := fakeKubeconform(t)
	shortenKubeconform(t)

	// Plant a "truncated" schema: a stall may leave one, and the retry must
	// not trust it.
	if err := os.MkdirAll(kubeconformCacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	planted := filepath.Join(kubeconformCacheDir, "half-written")
	if err := os.WriteFile(planted, []byte(`{"type":`), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runKubeconform(context.Background(), &Config{}, []byte(goodManifest))
	if err != nil {
		t.Fatalf("a single stall should be absorbed by the retry, got: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("a valid resource is not a failure, got %+v", out)
	}
	if _, err := os.Stat(planted); !os.IsNotExist(err) {
		t.Fatalf("the cache must be started over after a killed run; %s survived", planted)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(calls) != 2 {
		t.Fatalf("want exactly 2 kubeconform runs (stall, retry), got %d: %q", len(calls), calls)
	}
	for _, c := range calls {
		if !strings.Contains(c, "-cache "+kubeconformCacheDir) {
			t.Errorf("every run must reuse the schema cache, got args %q", c)
		}
	}
}

// Two stalls in a row are an outage, and the error still says so in the
// words the report has always used.
func TestKubeconformGivesUpAfterTwoStalls(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncat >/dev/null\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(dir, "kubeconform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	shortenKubeconform(t)

	start := time.Now()
	_, err := runKubeconform(context.Background(), &Config{}, []byte(goodManifest))
	if err == nil || !strings.Contains(err.Error(), "did not finish within") {
		t.Fatalf("want the deadline named, got %v", err)
	}
	if took := time.Since(start); took > 20*time.Second {
		t.Fatalf("two bounded attempts took %s; something is not being killed", took)
	}
}
