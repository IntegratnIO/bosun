package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/JamesAtIntegratnIO/bosun/childenv"
	"github.com/JamesAtIntegratnIO/bosun/redact"
)

// SchemaFailure is one rendered manifest the target cluster's schemas reject.
//
// The structured half of what ValidateManifests also writes as prose, and it
// exists because a count with nothing behind it is the shape this project
// spends its tests refusing. The report comment carries the sentences; a
// caller reading the verdict as data got `schema=3` and no way to learn which
// three, which reads exactly like a bug in whatever produced the number.
//
// Every field is text somebody else wrote: Kind and Name come out of a
// rendered manifest, Message out of kubeconform. Source is bosun's own key for
// the source that produced the stream, which is a config file's `name` or an
// Application's, so it is not bosun's either.
type SchemaFailure struct {
	// Source is which rendered stream the manifest came from, in the form the
	// report headings use: a source name, or "<source> on <cluster>".
	Source string `json:"source"`
	// Kind and Name identify the manifest inside that stream.
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Message is kubeconform's own verdict on it, verbatim.
	Message string `json:"message"`
}

// ValidateManifests renders every source and schema-validates the result with
// kubeconform, writing a markdown report of the findings to w. It returns the
// manifests that failed, one entry per rejection.
//
// The slice and the prose are two renderings of one pass rather than two
// passes: the report is what a person reads on the pull request, the slice is
// what a read surface publishes, and a gate that counted them separately would
// eventually disagree with itself about its own verdict.
//
// -ignore-missing-schemas is effectively mandatory rather than a convenience:
// CRDs outside the big projects are in no published catalogue, and without it
// one unknown kind fails a run that had nothing wrong with it. The cost is
// real and worth stating; those kinds are not checked.
func ValidateManifests(ctx context.Context, repoRoot string, cfg *Config, inv *Inventory, w io.Writer) ([]SchemaFailure, error) {
	if _, err := exec.LookPath("kubeconform"); err != nil {
		return nil, fmt.Errorf("kubeconform is not on PATH: %w", err)
	}

	streams, err := renderStreams(ctx, repoRoot, cfg, inv)
	if err != nil {
		return nil, err
	}

	var failures []SchemaFailure
	// Sorted, because the map above is ranged and this report goes into a
	// comment that is rewritten on every run: an unordered section list is a
	// diff between two runs that is not a difference in the manifests.
	names := make([]string, 0, len(streams))
	for name := range streams {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		out, err := runKubeconform(ctx, cfg, streams[name].manifests)
		if err != nil {
			return nil, fmt.Errorf("running kubeconform on %s: %w", name, err)
		}
		if len(out) > 0 {
			fmt.Fprintf(w, "### %s\n\n", name)
			for _, r := range out {
				failures = append(failures, SchemaFailure{
					Source: name, Kind: r.Kind, Name: r.Name, Message: r.Msg,
				})
				fmt.Fprintf(w, "- `%s/%s`: %s\n", r.Kind, r.Name, r.Msg)
			}
			fmt.Fprintln(w)
		}
	}

	// Never silent. A document this declined to validate is one a reader
	// might expect to have been checked, and the count is the only thing
	// standing between "skipped a values file" and "skipped your manifest".
	var skipped []string
	for _, name := range names {
		if n := streams[name].notManifests; n > 0 {
			skipped = append(skipped, fmt.Sprintf("%s (%d)", name, n))
		}
	}
	if len(skipped) > 0 {
		fmt.Fprintf(w, "Not validated, because they declare no `kind` and nothing would apply them: %s.\n\n",
			strings.Join(skipped, ", "))
	}

	if len(failures) == 0 {
		fmt.Fprintf(w, "All rendered manifests passed schema validation.\n")
	}
	return failures, nil
}

type kubeconformResult struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	Msg  string `json:"msg"`
}

// kubeconform fetches every schema over HTTP with a client that has no
// timeout of its own (v0.7.0: retryablehttp, RetryMax 2, no
// http.Client.Timeout), and without -cache it fetches them again for every
// source in every run -- dozens of requests per gate run to
// raw.githubusercontent.com for kinds whose schema has not changed. One
// response that stalls holds the whole process until the deadline, and the
// gate reports `could not run` on whichever source drew it. Seen on a live
// repository more than once as `kubeconform did not finish within 3m0s` on a
// source of two PrometheusRules, on pull requests that did not touch it.
//
// So: the downloads are cached on disk for the life of the process, which
// takes the network out of every run after the first for every schema that
// resolves, and a run that hits the deadline is tried once more on a fresh
// process and fresh connections before the gate gives up.
var (
	// kubeconformAttemptTimeout bounds one kubeconform process. A var only
	// so a test can shorten it.
	kubeconformAttemptTimeout = toolTimeout
	// kubeconformAttempts is how many processes one source may use. Two: a
	// stalled connection is the failure being absorbed, and a second stall
	// in a row is an outage the report should name rather than wait out.
	kubeconformAttempts = 2
	// kubeconformCacheDir is where downloaded schemas are kept. Under the
	// temp dir because that is the one place a read-only root filesystem
	// still leaves writable (the chart mounts an emptyDir there), and per
	// process lifetime is the right scope: a schema location is a moving
	// branch, and a restart picks its changes up.
	kubeconformCacheDir = filepath.Join(os.TempDir(), "bosun-kubeconform-cache")
)

func runKubeconform(ctx context.Context, cfg *Config, doc []byte) ([]kubeconformResult, error) {
	var lastErr error
	for attempt := 1; attempt <= kubeconformAttempts; attempt++ {
		out, err := runKubeconformOnce(ctx, cfg, doc)
		if err == nil {
			return out, nil
		}
		lastErr = err
		// Only the deadline is retried. An invalid manifest is a result,
		// and output kubeconform could not produce twice is not a stall.
		if !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return nil, err
		}
		// A process killed mid-download may have left a truncated schema
		// in the cache, and kubeconform would read it back as the schema
		// from then on: a manifest rejected by a file nobody wrote. Start
		// the cache over rather than trust any of it.
		_ = os.RemoveAll(kubeconformCacheDir)
	}
	return nil, lastErr
}

func runKubeconformOnce(ctx context.Context, cfg *Config, doc []byte) ([]kubeconformResult, error) {
	args := []string{"-strict", "-output", "json", "-summary=false"}
	// Best effort: without a cache directory kubeconform still works, it
	// only downloads everything again, which is how it ran before this.
	if err := os.MkdirAll(kubeconformCacheDir, 0o700); err == nil {
		args = append(args, "-cache", kubeconformCacheDir)
	}
	if cfg.Validate.IgnoreMissingSchemas {
		args = append(args, "-ignore-missing-schemas")
	}
	for _, s := range cfg.Validate.SchemaLocations {
		args = append(args, "-schema-location", s)
	}
	for _, k := range cfg.Validate.SkipKinds {
		args = append(args, "-skip", k)
	}
	args = append(args, "-")

	// Bounded like every other subprocess the gate starts. Not through `run`,
	// because that one reads a non-zero exit as a failure and this one must
	// not: kubeconform exits non-zero when a manifest is invalid, which is a
	// result rather than an execution failure, so the output is parsed either
	// way.
	ctx, cancel := context.WithTimeout(ctx, kubeconformAttemptTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubeconform", args...)
	// Like every other subprocess the gate starts: a document on stdin and
	// none of this process's credentials in its environment.
	cmd.Env = childenv.Environ()
	cmd.Stdin = bytes.NewReader(doc)
	// Kill promptly once the deadline passes, even if a grandchild holds the
	// pipes open.
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	// A killed kubeconform writes nothing, and the parse below would then
	// blame the empty stdout on kubeconform's output format. Say the deadline
	// expired instead.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("kubeconform did not finish within %s: %w", kubeconformAttemptTimeout, err)
	}

	var parsed struct {
		Resources []struct {
			Kind   string `json:"kind"`
			Name   string `json:"name"`
			Status string `json:"status"`
			Msg    string `json:"msg"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("%w (stderr: %s)", err, redact.Text(stderr.String()))
	}

	var bad []kubeconformResult
	for _, r := range parsed.Resources {
		if r.Status == "statusInvalid" || r.Status == "statusError" {
			bad = append(bad, kubeconformResult{Kind: r.Kind, Name: r.Name, Msg: r.Msg})
		}
	}
	return bad, nil
}

// stream is one source's manifests, plus what was left out of them.
type stream struct {
	manifests []byte
	// notManifests counts documents dropped for declaring no `kind`.
	notManifests int
}

// renderStreams re-collects every source and returns the raw manifests for
// each, keyed by a human-readable name.
//
// A document with no `kind` is not a manifest and is not offered to
// kubeconform. This is the gate agreeing with itself rather than a new
// tolerance: `objectFrom` already refuses one for the resource diff, so the
// same `values.yaml` sitting in a directory source was invisible to every
// finding except this one, where it arrived as `missing 'kind' key` and
// counted toward `Blockers.Schema` -- a bucket documented as "manifests the
// target cluster's schemas reject", with no repository-side remedy, blocking.
//
// Measured: two such files in the directory sources of a live repository put
// a standing `schema=2` on every pull request, on apps those pull requests
// did not touch, while ArgoCD reported both Applications Synced and Healthy.
// A check that is red on every change is a check people learn to override.
//
// Only the kindless case, and deliberately not every parse failure:
// kubeconform refusing a document that *has* a kind is a finding, and folding
// the two together is how this started.
func renderStreams(ctx context.Context, repoRoot string, cfg *Config, inv *Inventory) (map[string]stream, error) {
	out := map[string]stream{}
	for _, src := range cfg.Sources {
		batch, err := collect(ctx, repoRoot, cfg, inv, src)
		if err != nil {
			return nil, err
		}
		for _, d := range batch {
			key := d.source
			if d.cluster != nil {
				key = fmt.Sprintf("%s on %s", d.source, d.cluster.Name)
			}
			var buf bytes.Buffer
			st := out[key]
			for _, obj := range d.objects {
				if kind, _ := obj["kind"].(string); kind == "" {
					st.notManifests++
					continue
				}
				raw, err := yaml.Marshal(obj)
				if err != nil {
					return nil, err
				}
				buf.WriteString("---\n")
				buf.Write(raw)
			}
			st.manifests = append(st.manifests, buf.Bytes()...)
			out[key] = st
		}
	}
	return out, nil
}
