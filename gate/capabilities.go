package gate

import (
	"sort"
	"strings"
)

// Capabilities is what `helm template` is told about the cluster a chart is
// rendered for: the server version behind `.Capabilities.KubeVersion` and the
// API versions behind `.Capabilities.APIVersions.Has`.
//
// Without them helm falls back to the built-in defaults of whichever helm is
// installed, and those are not the cluster's. A chart that asks
// `.Capabilities.APIVersions.Has "policy/v1/PodDisruptionBudget"` gets "no"
// (helm's default set lists group/versions, never kinds), renders the
// policy/v1beta1 branch, and the gate then reports a policy/v1beta1 ->
// policy/v1 move that no cluster will ever see: base renders the fallback,
// head picks a different fallback, and the difference between them is helm's,
// not the change's. ArgoCD's repo-server passes both of these from its own
// cluster cache when it renders, so a gate that did not was diffing a chart
// against a cluster nothing deploys to.
type Capabilities struct {
	// KubeVersion is the server version as ArgoCD reports it, e.g. "1.36".
	KubeVersion string `json:"kubeVersion,omitempty"`
	// APIVersions are ArgoCD's list for the cluster, which carries both
	// `group/version` and `group/version/Kind` forms, the second being the
	// one charts actually test for.
	APIVersions []string `json:"apiVersions,omitempty"`
}

// empty is whether there is nothing to tell helm.
func (c Capabilities) empty() bool {
	return strings.TrimSpace(c.KubeVersion) == "" && len(c.APIVersions) == 0
}

// helmArgs is the `helm template` flags these capabilities become.
//
// Every value is checked against RefuseFlagLike and dropped rather than
// passed if it looks like a flag: they come from the ArgoCD API, not from the
// repository, but they still land in argv as bare tokens after `=`, and a
// value with a comma would be split by helm's string-slice parser into two,
// so those are dropped too.
func (c Capabilities) helmArgs() []string {
	var args []string
	if v := normaliseKubeVersion(c.KubeVersion); v != "" {
		args = append(args, "--kube-version", v)
	}
	seen := map[string]bool{}
	var versions []string
	for _, v := range c.APIVersions {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] || strings.ContainsAny(v, ", \t\n") || RefuseFlagLike("api version", v) != nil {
			continue
		}
		seen[v] = true
		versions = append(versions, v)
	}
	// Sorted so the argv, and therefore anything that logs it, is the same
	// on every run.
	sort.Strings(versions)
	if len(versions) > 0 {
		args = append(args, "--api-versions", strings.Join(versions, ","))
	}
	return args
}

// normaliseKubeVersion turns what a cluster reports into something helm's
// semver parser accepts. Managed distributions report minors like "30+", and
// helm refuses the whole render over the plus.
func normaliseKubeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return ""
	}
	if i := strings.IndexAny(v, "+-"); i >= 0 {
		v = v[:i]
	}
	for _, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			return ""
		}
	}
	if strings.Trim(v, ".") == "" {
		return ""
	}
	return "v" + v
}

// capabilityArgs is the helm flags for rendering into the named cluster.
//
// cluster is a Row's Cluster, which is a cluster name almost always and the
// server URL when the destination named a server the inventory did not know;
// the map is keyed by both. A cluster with nothing recorded renders exactly
// as it did before this existed, with helm's defaults, because inventing a
// version would be worse than admitting there is none.
func (c *Config) capabilityArgs(cluster string) []string {
	if c == nil || len(c.Capabilities) == 0 {
		return nil
	}
	if caps, ok := c.Capabilities[cluster]; ok && !caps.empty() {
		return caps.helmArgs()
	}
	return nil
}
