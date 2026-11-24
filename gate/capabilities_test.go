package gate

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The bug this exists for: rendering without the cluster's capabilities made
// `.Capabilities.APIVersions.Has "policy/v1/PodDisruptionBudget"` false, so a
// Kyverno bump reported a policy/v1beta1 -> policy/v1 move no cluster sees.
func TestCapabilitiesBecomeHelmFlags(t *testing.T) {
	got := Capabilities{
		KubeVersion: "1.36",
		APIVersions: []string{"v1", "policy/v1/PodDisruptionBudget", "policy/v1", "v1", " "},
	}.helmArgs()
	want := []string{"--kube-version", "v1.36", "--api-versions", "policy/v1,policy/v1/PodDisruptionBudget,v1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCapabilityValuesThatWouldBreakArgvAreDropped(t *testing.T) {
	got := Capabilities{
		KubeVersion: "--post-renderer=/tmp/x",
		APIVersions: []string{"--post-renderer=/tmp/x", "a/v1,b/v1", "apps/v1"},
	}.helmArgs()
	want := []string{"--api-versions", "apps/v1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKubeVersionIsNormalisedForHelm(t *testing.T) {
	for in, want := range map[string]string{
		"1.36": "v1.36", "v1.36.6": "v1.36.6", "1.30+": "v1.30",
		"1.29-eks": "v1.29", "": "", "garbage": "", ".": "",
	} {
		if got := normaliseKubeVersion(in); got != want {
			t.Errorf("normaliseKubeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnknownClusterRendersWithHelmDefaults(t *testing.T) {
	cfg := &Config{Capabilities: map[string]Capabilities{"hub": {KubeVersion: "1.36"}}}
	if got := cfg.capabilityArgs("elsewhere"); got != nil {
		t.Fatalf("a cluster with nothing recorded must not borrow another's: %q", got)
	}
	var nilCfg *Config
	if got := nilCfg.capabilityArgs("hub"); got != nil {
		t.Fatalf("nil config: %q", got)
	}
	if got := cfg.capabilityArgs("hub"); len(got) != 2 || got[1] != "v1.36" {
		t.Fatalf("got %q", got)
	}
}

// End to end through the chart-diff render: the helm argv carries the row's
// cluster capabilities.
func TestRenderChartVersionPassesTheClustersCapabilities(t *testing.T) {
	bin := t.TempDir()
	argvFile := filepath.Join(t.TempDir(), "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argvFile + "\n"
	if err := os.WriteFile(filepath.Join(bin, "helm"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := &Config{Capabilities: map[string]Capabilities{
		"the-cluster": {KubeVersion: "1.36", APIVersions: []string{"policy/v1/PodDisruptionBudget"}},
	}}
	_, err := renderChartVersion(context.Background(), cfg, t.TempDir(), Row{
		App: "kyverno", Cluster: "the-cluster", Chart: "kyverno",
		ChartRepo: "https://kyverno.github.io/kyverno", Version: "3.9.2",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatal(err)
	}
	argv := string(raw)
	for _, want := range []string{"--kube-version\nv1.36\n", "--api-versions\npolicy/v1/PodDisruptionBudget\n"} {
		if !strings.Contains(argv, want) {
			t.Errorf("helm argv lacks %q:\n%s", want, argv)
		}
	}
}
