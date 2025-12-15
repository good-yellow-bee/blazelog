package session

import (
	"testing"
	"time"
)

func TestStore_CreateAndGet(t *testing.T) {
	store := NewStore(time.Hour)

	session, err := store.Create("user-1", "admin", "admin")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if session.ID == "" {
		t.Error("session.ID is empty")
	}

	got, ok := store.Get(session.ID)
	if !ok {
		t.Error("Get() returned false, want true")
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", got.UserID, "user-1")
	}
}

func TestStore_GetExpired(t *testing.T) {
	store := NewStore(time.Millisecond)

	session, _ := store.Create("user-1", "admin", "admin")
	time.Sleep(5 * time.Millisecond)

	_, ok := store.Get(session.ID)
	if ok {
		t.Error("Get() returned true for expired session")
	}
}

func TestStore_Delete(t *testing.T) {
	store := NewStore(time.Hour)

	session, _ := store.Create("user-1", "admin", "admin")
	store.Delete(session.ID)

	_, ok := store.Get(session.ID)
	if ok {
		t.Error("Get() returned true after Delete()")
	}
}
