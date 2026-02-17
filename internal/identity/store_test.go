package identity

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "identities.enc")
	store, err := NewFileStore(path, []byte("test-master-password"))
	if err != nil {
		t.Fatalf("NewFileStore() error: %v", err)
	}
	return store
}

func TestStoreAddAndGet(t *testing.T) {
	store := newTestStore(t)
	id := Identity{
		Name:      "test-v2c",
		Version:   "2c",
		Community: "public",
	}
	if err := store.Add(id); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	got, err := store.Get("test-v2c")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Community != "public" {
		t.Errorf("expected community 'public', got %q", got.Community)
	}
}

func TestStoreList(t *testing.T) {
	store := newTestStore(t)
	store.Add(Identity{Name: "a", Version: "2c", Community: "x"})
	store.Add(Identity{Name: "b", Version: "3", Username: "user"})

	summaries, err := store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(summaries))
	}
}

func TestStoreRemove(t *testing.T) {
	store := newTestStore(t)
	store.Add(Identity{Name: "x", Version: "2c", Community: "test"})
	if err := store.Remove("x"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}
	_, err := store.Get("x")
	if err == nil {
		t.Error("expected error after removing identity")
	}
}

func TestStoreUpdate(t *testing.T) {
	store := newTestStore(t)
	store.Add(Identity{Name: "x", Version: "2c", Community: "old"})
	err := store.Update("x", Identity{Name: "x", Version: "2c", Community: "new"})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	got, _ := store.Get("x")
	if got.Community != "new" {
		t.Errorf("expected 'new', got %q", got.Community)
	}
}

func TestStorePersistence(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "identities.enc")
	password := []byte("test-password")

	// Create and add
	store1, _ := NewFileStore(path, password)
	store1.Add(Identity{Name: "persist", Version: "2c", Community: "test"})

	// Reopen with same password
	store2, err := NewFileStore(path, password)
	if err != nil {
		t.Fatalf("reopen error: %v", err)
	}
	got, err := store2.Get("persist")
	if err != nil {
		t.Fatalf("Get() after reopen error: %v", err)
	}
	if got.Community != "test" {
		t.Errorf("expected 'test', got %q", got.Community)
	}
}

func TestStoreWrongPassword(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "identities.enc")

	store1, _ := NewFileStore(path, []byte("correct"))
	store1.Add(Identity{Name: "x", Version: "2c", Community: "test"})

	_, err := NewFileStore(path, []byte("wrong"))
	if err == nil {
		t.Error("expected error with wrong password")
	}
}

func TestStoreDuplicateAdd(t *testing.T) {
	store := newTestStore(t)
	store.Add(Identity{Name: "dup", Version: "2c", Community: "a"})
	err := store.Add(Identity{Name: "dup", Version: "2c", Community: "b"})
	if err == nil {
		t.Error("expected error adding duplicate identity name")
	}
}
