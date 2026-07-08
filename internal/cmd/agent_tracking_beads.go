package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
)

// findCwdBeadsWorkDir finds the nearest .beads directory by walking up from CWD.
// It intentionally ignores BEADS_DIR for callers whose target is implied by
// the current rig worktree rather than inherited session environment.
func findCwdBeadsWorkDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	path := cwd
	for {
		if _, err := os.Stat(filepath.Join(path, ".beads")); err == nil {
			return path, nil
		}

		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}

	return "", fmt.Errorf("no .beads directory found")
}

// resolveAgentTrackingBeadsDir resolves the bead database used for agent state.
// Agent tracking follows the agent's current rig, so cwd-local redirects must
// win over an inherited town-level BEADS_DIR. The env-first resolver remains a
// fallback for contexts that do not have a cwd-local .beads directory.
func resolveAgentTrackingBeadsDir() (string, error) {
	workDir, err := findCwdBeadsWorkDir()
	if err != nil {
		workDir, err = findLocalBeadsDir()
	}
	if err != nil {
		return "", err
	}

	beadsDir := beads.ResolveBeadsDir(workDir)
	if beadsDir == "" {
		return "", fmt.Errorf("not in a beads workspace")
	}
	return beadsDir, nil
}

// resolveAgentStateBeadsDir resolves where a specific agent bead is actually
// stored. Rig-local agent beads win, but older/current towns may still have
// rig agent beads stranded in the town database. Patrol state commands must use
// the bead's storage DB consistently for all reads and writes.
func resolveAgentStateBeadsDir(agentBead, defaultBeadsDir string) (string, error) {
	defaultBeadsDir = beads.ResolveBeadsDir(defaultBeadsDir)
	var firstNotFound error

	for _, candidate := range agentStateCandidateBeadsDirs(defaultBeadsDir) {
		if _, err := getAllAgentLabels(agentBead, candidate); err == nil {
			return candidate, nil
		} else if isAgentBeadNotFoundError(err) {
			if firstNotFound == nil {
				firstNotFound = err
			}
			continue
		} else {
			return "", err
		}
	}

	if firstNotFound != nil {
		return "", firstNotFound
	}
	return "", fmt.Errorf("agent bead not found: %s", agentBead)
}

func agentStateCandidateBeadsDirs(defaultBeadsDir string) []string {
	var candidates []string
	seen := make(map[string]bool)
	add := func(dir string) {
		dir = beads.ResolveBeadsDir(dir)
		if dir == "" {
			return
		}
		clean := filepath.Clean(dir)
		if seen[clean] {
			return
		}
		seen[clean] = true
		candidates = append(candidates, clean)
	}

	add(defaultBeadsDir)

	if cwd, err := os.Getwd(); err == nil {
		if townRoot := beads.FindTownRoot(cwd); townRoot != "" {
			add(beads.GetTownBeadsPath(townRoot))
		}
	}
	if defaultBeadsDir != "" {
		if townRoot := beads.FindTownRoot(filepath.Dir(defaultBeadsDir)); townRoot != "" {
			add(beads.GetTownBeadsPath(townRoot))
		}
	}

	return candidates
}

func isAgentBeadNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "agent bead not found") || strings.Contains(msg, "not found")
}
