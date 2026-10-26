package state

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestSaveLoad(t *testing.T) {
	store := NewStore(t.TempDir())
	now := time.Now()
	rec := Record{PaneID: "%3", ServerPID: 42, Agent: "claude", State: Running, UpdatedAt: now, LastEvent: now}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(42, "%3")
	if err != nil || got == nil {
		t.Fatalf("Get: %v %v", got, err)
	}
	if got.State != Running || got.Agent != "claude" {
		t.Fatalf("got %+v", got)
	}

	all, err := store.Load(42)
	if err != nil || len(all) != 1 {
		t.Fatalf("Load: %d %v", len(all), err)
	}
}

func TestLoadIgnoresOtherServers(t *testing.T) {
	store := NewStore(t.TempDir())
	store.Save(Record{PaneID: "%1", ServerPID: 1, State: Idle})
	store.Save(Record{PaneID: "%1", ServerPID: 2, State: Running})

	got, _ := store.Load(2)
	if len(got) != 1 || got["%1"].State != Running {
		t.Fatalf("got %+v", got)
	}
}

func TestPruneServerDropsMissingPanes(t *testing.T) {
	store := NewStore(t.TempDir())
	store.Save(Record{PaneID: "%1", ServerPID: 5, State: Idle})
	store.Save(Record{PaneID: "%2", ServerPID: 5, State: Idle})

	store.PruneServer(5, map[string]bool{"%2": true})

	got, _ := store.Load(5)
	if len(got) != 1 {
		t.Fatalf("expected one record, got %d", len(got))
	}
	if _, ok := got["%2"]; !ok {
		t.Fatal("%2 should survive")
	}
	if _, err := os.Stat(filepath.Join(store.Root, "servers", "5", "%1.json")); !os.IsNotExist(err) {
		t.Fatal("%1 file should be gone")
	}
}

func TestPruneServersKeepsLive(t *testing.T) {
	store := NewStore(t.TempDir())
	current := os.Getpid()
	store.Save(Record{PaneID: "%1", ServerPID: current, State: Idle})
	store.Save(Record{PaneID: "%1", ServerPID: 999999, State: Idle})

	store.PruneServers(current)

	if _, err := os.Stat(filepath.Join(store.Root, "servers", "999999")); !os.IsNotExist(err) {
		t.Fatal("dead server dir should be removed")
	}
	if _, err := os.Stat(filepath.Join(store.Root, "servers", strconv.Itoa(current))); err != nil {
		t.Fatal("live server dir should stay")
	}
}

func TestDeleteMissingIsNotAnError(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Delete(1, "%9"); err != nil {
		t.Fatal(err)
	}
}
