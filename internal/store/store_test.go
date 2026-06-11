package store

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestCRUDAndPersistence(t *testing.T) {
	s, path := tempStore(t)

	added, err := s.Add(Item{Name: "Netflix-ish", Category: "subscription", Date: "2026-07-01", Cycle: "monthly", Cost: 15.49, RemindDays: 5})
	if err != nil {
		t.Fatal(err)
	}
	if added.ID == "" || added.CreatedAt.IsZero() {
		t.Fatalf("Add did not stamp ID/CreatedAt: %+v", added)
	}

	added.Cost = 17.99
	updated, err := s.Update(added.ID, added)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Cost != 17.99 || updated.CreatedAt != added.CreatedAt {
		t.Fatalf("update lost fields: %+v", updated)
	}

	// Reopen from disk: data must survive.
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	items := s2.Items()
	if len(items) != 1 || items[0].Cost != 17.99 {
		t.Fatalf("persistence failed: %+v", items)
	}

	if err := s2.Delete(items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s2.Delete(items[0].ID); err != ErrNotFound {
		t.Fatalf("double delete: want ErrNotFound, got %v", err)
	}
	if len(s2.Items()) != 0 {
		t.Fatal("delete did not remove item")
	}
}

func TestAddAllAtomic(t *testing.T) {
	s, _ := tempStore(t)
	_, err := s.AddAll([]Item{
		{Name: "ok", Category: "other", Date: "2026-07-01"},
		{Name: "", Category: "other", Date: "2026-07-01"}, // invalid
	})
	if err == nil {
		t.Fatal("AddAll accepted invalid batch")
	}
	if len(s.Items()) != 0 {
		t.Fatal("failed AddAll must not partially apply")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	s, path := tempStore(t)
	if s.Settings().Currency != "$" {
		t.Fatalf("default currency: %q", s.Settings().Currency)
	}
	if _, err := s.UpdateSettings(Settings{Currency: "€", Theme: "light", DefaultRemindDays: 30}); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.Settings(); got.Currency != "€" || got.Theme != "light" || got.DefaultRemindDays != 30 {
		t.Fatalf("settings did not persist: %+v", got)
	}
	if _, err := s.UpdateSettings(Settings{Currency: "", Theme: "dark"}); err == nil {
		t.Fatal("empty currency accepted")
	}
}

func TestOpenRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("corrupt file must not open silently")
	}
}
