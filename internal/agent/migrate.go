package agent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ishmukhamet/vps-watchdog/internal/state"
)

// stateVersion marks a state directory whose journal cursors were written by
// the agent itself (ADR 0007).
const (
	stateVersionFile = "state.version"
	stateVersion     = "2"
)

// migrateState runs once per state directory. Versions before 2 let
// journalctl write the cursor files, which happened only on rare clean exits,
// so those cursors can be months old; resuming from them would replay old
// login and sudo history as new alerts. They are discarded, and the watchers
// start at the current end of the journal, as on a fresh install.
func (a *Agent) migrateState() {
	path := a.state.Path(stateVersionFile)
	if data, err := state.ReadSmall(path, 16); err == nil && strings.TrimSpace(string(data)) == stateVersion {
		return
	}
	for _, name := range []string{"login.cursor", "sudo.cursor"} {
		if err := state.Remove(a.state.Path(name)); err != nil {
			a.log.Printf("state migration: remove %s: %v", name, err)
		} else {
			a.log.Printf("state migration: discarded %s written by an older version", name)
		}
	}
	// Empty leftovers of journalctl's own temporary files.
	if matches, err := filepath.Glob(a.state.Path(".#*.cursor*")); err == nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
	if err := state.WriteAtomic(path, []byte(stateVersion+"\n")); err != nil {
		a.log.Printf("state migration: save %s: %v", stateVersionFile, err)
	}
}
