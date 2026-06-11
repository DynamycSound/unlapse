// Package store implements unlapse's persistence layer: a single JSON file
// guarded by a mutex and written atomically, so user data is always a valid,
// human-readable document that can be backed up by copying one file.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const FileVersion = 1

var Categories = []string{
	"subscription", "warranty", "insurance", "document", "domain", "membership", "other",
}

var Cycles = []string{"none", "weekly", "monthly", "quarterly", "yearly", "custom"}

// Item is one tracked thing that expires or renews.
type Item struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Category   string    `json:"category"`
	Date       string    `json:"date"`                // YYYY-MM-DD anchor: next expiry/renewal when created
	Cycle      string    `json:"cycle"`               // none = one-time expiry
	CycleDays  int       `json:"cycleDays,omitempty"` // only for cycle == "custom"
	Cost       float64   `json:"cost"`                // per cycle; 0 = free
	RemindDays int       `json:"remindDays"`
	URL        string    `json:"url,omitempty"`
	Notes      string    `json:"notes,omitempty"`
	Archived   bool      `json:"archived"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Settings struct {
	Currency          string `json:"currency"`
	Theme             string `json:"theme"`
	DefaultRemindDays int    `json:"defaultRemindDays"`
}

type fileDoc struct {
	Version  int      `json:"version"`
	Settings Settings `json:"settings"`
	Items    []Item   `json:"items"`
}

type Store struct {
	mu       sync.Mutex
	path     string
	settings Settings
	items    []Item
}

var ErrNotFound = errors.New("item not found")

func defaultSettings() Settings {
	return Settings{Currency: "$", Theme: "dark", DefaultRemindDays: 14}
}

// Open loads the data file at path, creating its directory if needed.
// A missing file is not an error: the store starts empty.
func Open(path string) (*Store, error) {
	s := &Store{path: path, settings: defaultSettings()}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read data file: %w", err)
	}
	var doc fileDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse data file %s: %w", path, err)
	}
	if doc.Settings.Currency != "" {
		s.settings = doc.Settings
	}
	s.items = doc.Items
	return s, nil
}

// save writes the document atomically: temp file in the same directory, then rename.
// Callers must hold s.mu.
func (s *Store) save() error {
	doc := fileDoc{Version: FileVersion, Settings: s.settings, Items: s.items}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".unlapse-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is unrecoverable in practice; fall back to time.
		return fmt.Sprintf("t%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// Validate checks an item's user-supplied fields, normalizing in place.
func (it *Item) Validate() error {
	it.Name = strings.TrimSpace(it.Name)
	if it.Name == "" {
		return errors.New("name is required")
	}
	if len(it.Name) > 200 {
		return errors.New("name is too long (max 200 characters)")
	}
	if !contains(Categories, it.Category) {
		return fmt.Errorf("unknown category %q", it.Category)
	}
	if _, err := time.Parse("2006-01-02", it.Date); err != nil {
		return fmt.Errorf("date must be YYYY-MM-DD, got %q", it.Date)
	}
	if it.Cycle == "" {
		it.Cycle = "none"
	}
	if !contains(Cycles, it.Cycle) {
		return fmt.Errorf("unknown cycle %q", it.Cycle)
	}
	if it.Cycle == "custom" {
		if it.CycleDays < 1 || it.CycleDays > 3650 {
			return errors.New("custom cycle needs cycleDays between 1 and 3650")
		}
	} else {
		it.CycleDays = 0
	}
	if it.Cost < 0 {
		return errors.New("cost cannot be negative")
	}
	if it.RemindDays < 0 || it.RemindDays > 365 {
		return errors.New("remindDays must be between 0 and 365")
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *Store) Items() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Item, len(s.items))
	copy(out, s.items)
	return out
}

func (s *Store) Get(id string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range s.items {
		if it.ID == id {
			return it, nil
		}
	}
	return Item{}, ErrNotFound
}

func (s *Store) Add(it Item) (Item, error) {
	if err := it.Validate(); err != nil {
		return Item{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	it.ID = newID()
	it.CreatedAt = now
	it.UpdatedAt = now
	s.items = append(s.items, it)
	if err := s.save(); err != nil {
		s.items = s.items[:len(s.items)-1]
		return Item{}, err
	}
	return it, nil
}

func (s *Store) Update(id string, upd Item) (Item, error) {
	if err := upd.Validate(); err != nil {
		return Item{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, it := range s.items {
		if it.ID != id {
			continue
		}
		upd.ID = it.ID
		upd.CreatedAt = it.CreatedAt
		upd.UpdatedAt = time.Now().UTC()
		prev := s.items[i]
		s.items[i] = upd
		if err := s.save(); err != nil {
			s.items[i] = prev
			return Item{}, err
		}
		return upd, nil
	}
	return Item{}, ErrNotFound
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, it := range s.items {
		if it.ID != id {
			continue
		}
		prev := s.items
		s.items = append(append([]Item{}, s.items[:i]...), s.items[i+1:]...)
		if err := s.save(); err != nil {
			s.items = prev
			return err
		}
		return nil
	}
	return ErrNotFound
}

// AddAll inserts items in one save, used by CSV import and sample data.
func (s *Store) AddAll(items []Item) (int, error) {
	for i := range items {
		if err := items[i].Validate(); err != nil {
			return 0, fmt.Errorf("item %d (%s): %w", i+1, items[i].Name, err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	prev := s.items
	for _, it := range items {
		it.ID = newID()
		it.CreatedAt = now
		it.UpdatedAt = now
		s.items = append(s.items, it)
	}
	if err := s.save(); err != nil {
		s.items = prev
		return 0, err
	}
	return len(items), nil
}

func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Store) UpdateSettings(set Settings) (Settings, error) {
	set.Currency = strings.TrimSpace(set.Currency)
	if set.Currency == "" || len(set.Currency) > 8 {
		return Settings{}, errors.New("currency symbol must be 1-8 characters")
	}
	if set.Theme != "dark" && set.Theme != "light" {
		return Settings{}, errors.New("theme must be dark or light")
	}
	if set.DefaultRemindDays < 0 || set.DefaultRemindDays > 365 {
		return Settings{}, errors.New("defaultRemindDays must be between 0 and 365")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.settings
	s.settings = set
	if err := s.save(); err != nil {
		s.settings = prev
		return Settings{}, err
	}
	return set, nil
}

// SortByNext orders items by their next occurrence relative to today.
func SortByNext(items []Item, today time.Time) {
	sort.SliceStable(items, func(i, j int) bool {
		ni, _ := NextOccurrence(items[i], today)
		nj, _ := NextOccurrence(items[j], today)
		if ni.Equal(nj) {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		return ni.Before(nj)
	})
}
