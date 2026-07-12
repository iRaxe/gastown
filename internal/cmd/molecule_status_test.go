package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestOutputMoleculeStatus_StandaloneFormulaShowsVars(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir tempDir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	status := MoleculeStatusInfo{
		HasWork:         true,
		PinnedBead:      &beads.Issue{ID: "gt-wisp-xyz", Title: "Standalone formula work"},
		AttachedFormula: "mol-release",
		AttachedVars:    []string{"version=1.2.3", "channel=stable"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputMoleculeStatus(status)

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = oldStdout
	output := buf.String()

	if !strings.Contains(output, "📐 Formula: mol-release") {
		t.Fatalf("expected formula in output, got:\n%s", output)
	}
	if !strings.Contains(output, "--var version=1.2.3") || !strings.Contains(output, "--var channel=stable") {
		t.Fatalf("expected formula vars in output, got:\n%s", output)
	}
}

func TestOutputMoleculeStatus_FormulaWispShowsWorkflowContext(t *testing.T) {
	status := MoleculeStatusInfo{
		HasWork:         true,
		PinnedBead:      &beads.Issue{ID: "tool-wisp-demo", Title: "demo-hello"},
		AttachedFormula: "demo-hello",
		Progress: &MoleculeProgressInfo{
			RootID:     "tool-wisp-demo",
			RootTitle:  "demo-hello",
			TotalSteps: 3,
			DoneSteps:  0,
			ReadySteps: []string{"tool-wisp-step-1"},
		},
		NextAction: "Show the workflow steps: gt prime or bd mol current tool-wisp-demo",
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputMoleculeStatus(status)

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = oldStdout
	output := buf.String()

	if !strings.Contains(output, "📐 Formula: demo-hello") {
		t.Fatalf("expected formula line in output, got:\n%s", output)
	}
	if strings.Contains(output, "No molecule attached") {
		t.Fatalf("formula wisp should not be rendered as naked work, got:\n%s", output)
	}
	if strings.Contains(output, "Attach a molecule to start work") {
		t.Fatalf("formula wisp should not suggest gt mol attach, got:\n%s", output)
	}
	if !strings.Contains(output, "Show the workflow steps: gt prime or bd mol current tool-wisp-demo") {
		t.Fatalf("expected workflow next action, got:\n%s", output)
	}
}

func TestSelectCurrentWorkflowPrefersAuthoritativeHook(t *testing.T) {
	hooked := &beads.Issue{
		ID:          "hq-wisp-patrol",
		Title:       "mol-witness-patrol",
		Description: "attached_molecule: hq-wisp-patrol\nattached_formula: mol-witness-patrol",
	}
	legacyHandoff := &beads.Issue{
		ID:          "mw-handoff",
		Title:       "Witness Handoff",
		Description: "attached_molecule: hq-wisp-stale",
	}

	source, moleculeID := selectCurrentWorkflow(hooked, legacyHandoff)
	if source != hooked {
		t.Fatalf("source = %#v, want authoritative hooked patrol", source)
	}
	if moleculeID != hooked.ID {
		t.Fatalf("moleculeID = %q, want %q", moleculeID, hooked.ID)
	}
}

func TestSelectCurrentWorkflowUsesStandaloneFormulaRoot(t *testing.T) {
	hooked := &beads.Issue{
		ID:          "hq-wisp-formula",
		Title:       "mol-witness-patrol",
		Description: "attached_formula: mol-witness-patrol",
	}

	source, moleculeID := selectCurrentWorkflow(hooked, nil)
	if source != hooked {
		t.Fatalf("source = %#v, want hooked formula root", source)
	}
	if moleculeID != hooked.ID {
		t.Fatalf("moleculeID = %q, want standalone root %q", moleculeID, hooked.ID)
	}
}

func TestResolveMoleculeWorkDirRoutesHQRootFromRig(t *testing.T) {
	townRoot := t.TempDir()
	beadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir town beads: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(beadsDir, "routes.jsonl"),
		[]byte("{\"prefix\":\"hq-\",\"path\":\".\"}\n"),
		0o644,
	); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	rigWorkDir := filepath.Join(townRoot, "testrig")
	if got := resolveMoleculeWorkDir(townRoot, rigWorkDir, "hq-wisp-patrol"); got != townRoot {
		t.Fatalf("resolveMoleculeWorkDir() = %q, want town root %q", got, townRoot)
	}
}

func TestGetMoleculeProgressInfoIncludesEphemeralSteps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock bd script uses POSIX shell")
	}

	binDir := t.TempDir()
	bdScript := `#!/bin/sh
case "$*" in
  *"show hq-wisp-patrol --json"*)
    printf '%s\n' '[{"id":"hq-wisp-patrol","title":"mol-witness-patrol","status":"hooked","ephemeral":true}]'
    ;;
  *"list --json"*"--parent=hq-wisp-patrol"*)
    printf '%s\n' '[]'
    ;;
  *"query --json"*"parent=\"hq-wisp-patrol\""*)
    printf '%s\n' '[{"id":"hq-wisp-step","title":"inbox-check","status":"closed","parent":"hq-wisp-patrol","ephemeral":true}]'
    ;;
  *)
    printf '%s\n' '[]'
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(bdScript), 0o755); err != nil {
		t.Fatalf("write fake bd: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	beads.ResetBdAllowStaleCacheForTest()
	t.Cleanup(beads.ResetBdAllowStaleCacheForTest)

	progress, err := getMoleculeProgressInfo(beads.New(t.TempDir()), "hq-wisp-patrol")
	if err != nil {
		t.Fatalf("getMoleculeProgressInfo: %v", err)
	}
	if progress == nil || progress.TotalSteps != 1 || progress.DoneSteps != 1 {
		t.Fatalf("progress = %#v, want one completed ephemeral step", progress)
	}
}

func TestRunMoleculeProgressRoutesHQEphemeralStepsFromRig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock bd script uses POSIX shell")
	}

	townRoot := t.TempDir()
	rigRoot := filepath.Join(townRoot, "testrig")
	for _, dir := range []string{
		filepath.Join(townRoot, ".beads"),
		filepath.Join(townRoot, "mayor"),
		filepath.Join(rigRoot, ".beads"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(townRoot, "mayor", "town.json"), []byte(`{"name":"test-town"}`), 0o644); err != nil {
		t.Fatalf("write town.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(townRoot, ".beads", "routes.jsonl"), []byte("{\"prefix\":\"hq-\",\"path\":\".\"}\n"), 0o644); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	binDir := t.TempDir()
	bdScript := `#!/bin/sh
case "$*" in
  *"version"*)
    printf '%s\n' 'bd version 0.65.0'
    ;;
  *"show hq-wisp-patrol --json"*)
    [ "$BEADS_DIR" = "$TOWN_BEADS" ] || { echo "wrong database: $BEADS_DIR" >&2; exit 1; }
    printf '%s\n' '[{"id":"hq-wisp-patrol","title":"mol-witness-patrol","status":"hooked","ephemeral":true}]'
    ;;
  *"list --json"*"--parent=hq-wisp-patrol"*)
    [ "$BEADS_DIR" = "$TOWN_BEADS" ] || { echo "wrong database: $BEADS_DIR" >&2; exit 1; }
    printf '%s\n' '[]'
    ;;
  *"query --json"*"parent=\"hq-wisp-patrol\""*)
    [ "$BEADS_DIR" = "$TOWN_BEADS" ] || { echo "wrong database: $BEADS_DIR" >&2; exit 1; }
    printf '%s\n' '[{"id":"hq-wisp-step","title":"inbox-check","status":"closed","parent":"hq-wisp-patrol","ephemeral":true}]'
    ;;
  *)
    printf '%s\n' '[]'
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(bdScript), 0o755); err != nil {
		t.Fatalf("write fake bd: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BEADS_DIR", filepath.Join(rigRoot, ".beads"))
	t.Setenv("TOWN_BEADS", filepath.Join(townRoot, ".beads"))
	t.Setenv("GT_TOWN_ROOT", townRoot)
	t.Chdir(rigRoot)
	beads.ResetBdAllowStaleCacheForTest()
	t.Cleanup(beads.ResetBdAllowStaleCacheForTest)

	if err := runMoleculeProgress(nil, []string{"hq-wisp-patrol"}); err != nil {
		t.Fatalf("runMoleculeProgress: %v", err)
	}
}

func TestFindHookedWorkForTargetReturnsQueryError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock bd script uses POSIX shell")
	}
	binDir := t.TempDir()
	bdScript := `#!/bin/sh
case "$*" in
  *"version"*) printf '%s\n' 'bd version 0.65.0' ;;
  *) printf '%s\n' 'simulated query failure' >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(bdScript), 0o755); err != nil {
		t.Fatalf("write fake bd: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	beads.ResetBdAllowStaleCacheForTest()
	t.Cleanup(beads.ResetBdAllowStaleCacheForTest)

	_, err := findHookedWorkForTarget(beads.New(t.TempDir()), "testrig/witness", "")
	if err == nil || !strings.Contains(err.Error(), "simulated query failure") {
		t.Fatalf("error = %v, want query failure", err)
	}
}
