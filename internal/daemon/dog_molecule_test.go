package daemon

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type testLogger struct {
	t *testing.T
}

func (l testLogger) Printf(format string, args ...interface{}) {
	l.t.Helper()
	l.t.Logf(format, args...)
}

func TestParseWispID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID string
	}{
		{
			name:   "standard wisp output",
			input:  "✓ Spawned wisp: gt-wisp-abc123 — Reap stale wisps",
			wantID: "gt-wisp-abc123",
		},
		{
			name:   "wisp ID with ANSI codes",
			input:  "\033[32m✓\033[0m Spawned wisp: \033[1mgt-wisp-xyz789\033[0m — Title",
			wantID: "gt-wisp-xyz789",
		},
		{
			name:   "empty output",
			input:  "",
			wantID: "",
		},
		{
			name:   "no wisp ID in output",
			input:  "Error: something went wrong",
			wantID: "",
		},
		{
			name:   "wisp ID at end of line",
			input:  "Created gt-wisp-def456",
			wantID: "gt-wisp-def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWispID(tt.input)
			if got != tt.wantID {
				t.Errorf("parseWispID(%q) = %q, want %q", tt.input, got, tt.wantID)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ANSI", "hello", "hello"},
		{"color code", "\033[32mgreen\033[0m", "green"},
		{"bold", "\033[1mbold\033[0m", "bold"},
		{"multiple codes", "\033[32m✓\033[0m \033[1mtext\033[0m", "✓ text"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseChildrenJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "bare array",
			input:     `[{"id":"a","title":"Probe","status":"open"}]`,
			wantCount: 1,
		},
		{
			name:      "map wrapper from bd show",
			input:     `{"hq-wisp-root":[{"id":"hq-wisp-a","title":"Probe","status":"open"},{"id":"hq-wisp-b","title":"Report","status":"open"}]}`,
			wantCount: 2,
		},
		{
			name:      "map wrapper with schema metadata from bd show",
			input:     `{"hq-wisp-root":[{"id":"hq-wisp-a","title":"Probe","status":"open"}],"schema_version":1}`,
			wantCount: 1,
		},
		{
			name:      "empty map wrapper",
			input:     `{"hq-wisp-root":[]}`,
			wantCount: 0,
		},
		{
			name:      "empty array",
			input:     `[]`,
			wantCount: 0,
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChildrenJSON(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(got) != tt.wantCount {
				t.Errorf("got %d children, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestDogMolGracefulDegradation(t *testing.T) {
	// A dogMol with empty rootID should be a no-op for all operations.
	dm := &dogMol{
		rootID:  "",
		stepIDs: make(map[string]string),
	}

	// These should not panic or error — graceful degradation.
	dm.closeStep("scan")
	dm.failStep("scan", "test failure")
	dm.close()
}

func TestCloseRemainingStepsRetriesDependencyBlockedChildren(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "closed.txt")
	bdPath := filepath.Join(dir, "bd")
	script := `#!/usr/bin/env bash
set -euo pipefail

state="${FAKE_BD_STATE:?}"

if [[ "$1" == "show" ]]; then
  cat <<'JSON'
{"hq-wisp-root":[
  {"id":"report","title":"Report findings and return to kennel","status":"open"},
  {"id":"auto","title":"Auto-close stale issues","status":"open"},
  {"id":"reap","title":"Reap stale wisps","status":"open"},
  {"id":"scan","title":"Scan databases for reaper candidates","status":"open"}
],"schema_version":1}
JSON
  exit 0
fi

if [[ "$1" == "close" ]]; then
  id="$2"
  touch "$state"

  has_closed() {
    grep -qx "$1" "$state"
  }

  mark_closed() {
    has_closed "$1" || echo "$1" >> "$state"
  }

  case "$id" in
    scan)
      mark_closed scan
      ;;
    reap)
      has_closed scan || { echo "cannot close reap: blocked by open issues [scan]" >&2; exit 1; }
      mark_closed reap
      ;;
    auto)
      has_closed reap || { echo "cannot close auto: blocked by open issues [reap]" >&2; exit 1; }
      mark_closed auto
      ;;
    report)
      has_closed auto || { echo "cannot close report: blocked by open issues [auto]" >&2; exit 1; }
      mark_closed report
      ;;
    *)
      echo "unexpected close $id" >&2
      exit 1
      ;;
  esac
  exit 0
fi

echo "unexpected command $*" >&2
exit 1
`
	if err := os.WriteFile(bdPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_BD_STATE", statePath)

	dm := &dogMol{
		rootID:   "hq-wisp-root",
		bdPath:   bdPath,
		townRoot: dir,
		logger:   testLogger{t: t},
	}

	dm.closeRemainingSteps()

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(string(data))
	sort.Strings(got)
	want := []string{"auto", "reap", "report", "scan"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("closed steps = %v, want %v", got, want)
	}
}
