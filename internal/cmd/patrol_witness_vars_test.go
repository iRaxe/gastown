package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

const witnessPatrolAllOK = "inbox-check:OK,process-cleanups:OK,check-refinery:OK,survey-workers:OK,check-timer-gates:OK,check-swarm-completion:OK,patrol-cleanup:OK,context-check:OK,loop-or-exit:OK"

func TestRunPatrolNewWitnessInjectsRigAndPrefix(t *testing.T) {
	witnessDir, commandLog := setupWitnessPatrolCommandTest(t, false)
	t.Chdir(witnessDir)

	oldRole := patrolNewRole
	patrolNewRole = ""
	t.Cleanup(func() { patrolNewRole = oldRole })

	if err := runPatrolNew(nil, nil); err != nil {
		t.Fatalf("runPatrolNew: %v", err)
	}
	assertWitnessPatrolVarsInSpawn(t, commandLog)
}

func TestRunPatrolReportWitnessInjectsRigAndPrefixIntoSuccessor(t *testing.T) {
	witnessDir, commandLog := setupWitnessPatrolCommandTest(t, true)
	t.Chdir(witnessDir)

	oldSummary, oldSteps := patrolReportSummary, patrolReportSteps
	patrolReportSummary = "witness vars regression"
	patrolReportSteps = witnessPatrolAllOK
	t.Cleanup(func() {
		patrolReportSummary, patrolReportSteps = oldSummary, oldSteps
	})

	if err := runPatrolReport(nil, nil); err != nil {
		t.Fatalf("runPatrolReport: %v", err)
	}
	assertWitnessPatrolVarsInSpawn(t, commandLog)
}

func setupWitnessPatrolCommandTest(t *testing.T, activePatrol bool) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mock command scripts use POSIX shell")
	}

	townRoot := t.TempDir()
	witnessDir := filepath.Join(townRoot, "wavo_hub", "witness")
	for _, dir := range []string{
		filepath.Join(townRoot, ".beads"),
		filepath.Join(townRoot, "mayor"),
		witnessDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(townRoot, "mayor", "town.json"), []byte(`{"name":"test-town"}`), 0o644); err != nil {
		t.Fatalf("write town.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(townRoot, ".beads", "routes.jsonl"), []byte("{\"prefix\":\"hq-\",\"path\":\".\"}\n{\"prefix\":\"hub-\",\"path\":\"wavo_hub\"}\n"), 0o644); err != nil {
		t.Fatalf("write routes.jsonl: %v", err)
	}

	binDir := t.TempDir()
	commandLog := filepath.Join(t.TempDir(), "bd.log")
	bdScript := `#!/bin/sh
printf '%s\n' "$*" >> "$BD_LOG"
if [ "$ACTIVE_PATROL" = "1" ]; then
  case "$*" in
    *"show hub-wisp-current --json"*)
      printf '%s\n' '[{"id":"hub-wisp-current","title":"mol-witness-patrol (wisp)","status":"hooked","assignee":"wavo_hub/witness","description":"attached_molecule: hub-wisp-current\nattached_formula: mol-witness-patrol","updated_at":"2026-07-13T00:00:00Z","ephemeral":true}]'
      exit 0
      ;;
    *"--parent=hub-wisp-current"*|*"parent=\"hub-wisp-current\""*)
      printf '%s\n' '[]'
      exit 0
      ;;
	*"--assignee=wavo_hub/witness"*"--status=hooked"*|*"assignee=\"wavo_hub/witness\""*"status=\"hooked\""*|*"status=\"hooked\""*"assignee=\"wavo_hub/witness\""*)
      printf '%s\n' '[{"id":"hub-wisp-current","title":"mol-witness-patrol (wisp)","status":"hooked","assignee":"wavo_hub/witness","updated_at":"2026-07-13T00:00:00Z","ephemeral":true}]'
      exit 0
      ;;
  esac
fi
case "$*" in
  *"mol wisp create"*)
    printf '%s\n' 'Root issue: hub-wisp-successor'
    ;;
  *)
    printf '%s\n' '[]'
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(bdScript), 0o755); err != nil {
		t.Fatalf("write fake bd: %v", err)
	}
	gtScript := `#!/bin/sh
case "$*" in
  "formula list") printf '%s\n' 'mol-witness-patrol  Witness patrol' ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "gt"), []byte(gtScript), 0o755); err != nil {
		t.Fatalf("write fake gt: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BD_LOG", commandLog)
	t.Setenv("ACTIVE_PATROL", map[bool]string{false: "", true: "1"}[activePatrol])
	t.Setenv("GT_TEST_ATTACHED_MOLECULE_LOG", filepath.Join(t.TempDir(), "attachment.log"))
	t.Setenv("GT_ROLE", "")
	t.Setenv("GT_RIG", "")
	t.Setenv("GT_TOWN_ROOT", townRoot)
	t.Setenv("GT_ROOT", townRoot)
	beads.ResetBdAllowStaleCacheForTest()
	t.Cleanup(beads.ResetBdAllowStaleCacheForTest)

	return witnessDir, commandLog
}

func assertWitnessPatrolVarsInSpawn(t *testing.T, commandLog string) {
	t.Helper()
	data, err := os.ReadFile(commandLog)
	if err != nil {
		t.Fatalf("read bd command log: %v", err)
	}
	log := string(data)
	for _, want := range []string{"--var rig=wavo_hub", "--var prefix=hub"} {
		if !strings.Contains(log, want) {
			t.Fatalf("patrol spawn missing %q:\n%s", want, log)
		}
	}
}
