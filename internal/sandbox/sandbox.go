// Package sandbox is the environment a project's own commands run in when
// the framework runs them — its tests, its build, an agent's check: the
// person's own, but with a home of its own, so that nothing those commands
// write lands in the person's configuration. Tests written by an agent can
// write anywhere their home leads; on 11-23 one wrote the person's
// models.json.
package sandbox

import (
	"os"
	"os/exec"
	"strings"
	"sync"
)

var (
	once sync.Once
	env  []string
)

// Env is the environment a project's command runs in: the person's, its
// home a fresh folder of its own, and the Go toolchain's caches where they
// were — found from the person's home — so that nothing is fetched or built
// again for want of them.
func Env() []string {
	once.Do(func() {
		home, err := os.MkdirTemp("", "iflow-home-")
		if err != nil {
			env = os.Environ()
			return
		}
		keep := map[string]string{}
		if out, err := exec.Command("go", "env", "GOPATH", "GOMODCACHE", "GOCACHE").Output(); err == nil {
			v := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(v) == 3 {
				keep["GOPATH"], keep["GOMODCACHE"], keep["GOCACHE"] = v[0], v[1], v[2]
			}
		}
		keep["HOME"], keep["XDG_CONFIG_HOME"] = home, home+"/.config"
		for _, kv := range os.Environ() {
			k, _, _ := strings.Cut(kv, "=")
			if _, set := keep[k]; !set {
				env = append(env, kv)
			}
		}
		for k, v := range keep {
			if v != "" {
				env = append(env, k+"="+v)
			}
		}
	})
	return env
}
