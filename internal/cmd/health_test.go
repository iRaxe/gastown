package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheckDatabaseHealthUsesDiscoveredDatabases(t *testing.T) {
	townRoot := t.TempDir()
	want := []string{"app_mywavo", "easycom", "hq", "mywavo_chatwoot", "wavo_hub"}
	for _, name := range want {
		writeDoltManifest(t, townRoot, name)
	}
	writeBeadsMetadata(t, filepath.Join(townRoot, ".beads"), "hq")
	for _, name := range []string{"app_mywavo", "easycom", "mywavo_chatwoot", "wavo_hub"} {
		writeBeadsMetadata(t, filepath.Join(townRoot, name, ".beads"), name)
	}
	writeDoltManifest(t, townRoot, "beads")

	health := checkDatabaseHealth(townRoot, 1)

	var got []string
	for _, db := range health {
		got = append(got, db.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("database names = %v, want %v", got, want)
	}
	for _, unwanted := range []string{"beads", "gt", "mo"} {
		for _, name := range got {
			if name == unwanted {
				t.Fatalf("database names included nonexistent database %q: %v", unwanted, got)
			}
		}
	}
}

func writeDoltManifest(t *testing.T, townRoot, dbName string) {
	t.Helper()

	manifestPath := filepath.Join(townRoot, ".dolt-data", dbName, ".dolt", "noms", "manifest")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("create manifest dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("manifest"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writeBeadsMetadata(t *testing.T, beadsDir, dbName string) {
	t.Helper()

	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("create beads dir: %v", err)
	}
	metadata := []byte(`{"dolt_database":"` + dbName + `"}`)
	if err := os.WriteFile(filepath.Join(beadsDir, "metadata.json"), metadata, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
}
