// Package server exposes unlapse's REST API and the embedded web UI.
package server

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/DynamycSound/unlapse/internal/store"
)

//go:embed all:webdist
var webFS embed.FS

// Version is stamped at build time via -ldflags.
var Version = "dev"

type Server struct {
	store *store.Store
	mux   *http.ServeMux
	now   func() time.Time
}

func New(st *store.Store) *Server {
	s := &Server{store: st, mux: http.NewServeMux(), now: time.Now}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/items", s.handleListItems)
	s.mux.HandleFunc("POST /api/items", s.handleCreateItem)
	s.mux.HandleFunc("PUT /api/items/{id}", s.handleUpdateItem)
	s.mux.HandleFunc("DELETE /api/items/{id}", s.handleDeleteItem)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	s.mux.HandleFunc("GET /api/export.csv", s.handleExportCSV)
	s.mux.HandleFunc("POST /api/import", s.handleImportCSV)
	s.mux.HandleFunc("POST /api/sample", s.handleSample)
	s.mux.HandleFunc("GET /calendar.ics", s.handleICS)

	sub, err := fs.Sub(webFS, "webdist")
	if err != nil {
		log.Fatalf("embedded web assets missing: %v", err)
	}
	s.mux.Handle("GET /", http.FileServerFS(sub))
}

// itemView is an Item plus fields computed for the current day.
type itemView struct {
	store.Item
	NextDate    string  `json:"nextDate"`
	DaysLeft    int     `json:"daysLeft"`
	Status      string  `json:"status"`
	MonthlyCost float64 `json:"monthlyCost"`
	YearlyCost  float64 `json:"yearlyCost"`
}

func (s *Server) view(it store.Item) itemView {
	today := s.now().UTC()
	next, _ := store.NextOccurrence(it, today)
	return itemView{
		Item:        it,
		NextDate:    next.Format("2006-01-02"),
		DaysLeft:    store.DaysUntil(next, today),
		Status:      string(store.StatusFor(it, today)),
		MonthlyCost: round2(store.MonthlyCost(it)),
		YearlyCost:  round2(store.YearlyCost(it)),
	}
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	items := s.store.Items()
	if r.URL.Query().Get("includeArchived") != "1" {
		active := items[:0]
		for _, it := range items {
			if !it.Archived {
				active = append(active, it)
			}
		}
		items = active
	}
	store.SortByNext(items, s.now().UTC())
	views := make([]itemView, 0, len(items))
	for _, it := range items {
		views = append(views, s.view(it))
	}
	writeJSON(w, http.StatusOK, views)
}

func decodeItem(r *http.Request) (store.Item, error) {
	var it store.Item
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&it); err != nil {
		return it, errors.New("invalid request body: " + err.Error())
	}
	return it, nil
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	it, err := decodeItem(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.store.Add(it)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.view(saved))
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	it, err := decodeItem(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.store.Update(r.PathValue("id"), it)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view(saved))
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	err := s.store.Delete(r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type statsResponse struct {
	Currency       string             `json:"currency"`
	ActiveItems    int                `json:"activeItems"`
	MonthlyCost    float64            `json:"monthlyCost"`
	YearlyCost     float64            `json:"yearlyCost"`
	Overdue        int                `json:"overdue"`
	DueSoon        int                `json:"dueSoon"`
	Next30Days     int                `json:"next30Days"`
	ByCategory     map[string]int     `json:"byCategory"`
	CostByCategory map[string]float64 `json:"costByCategory"`
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	today := s.now().UTC()
	resp := statsResponse{
		Currency:       s.store.Settings().Currency,
		ByCategory:     map[string]int{},
		CostByCategory: map[string]float64{},
	}
	for _, it := range s.store.Items() {
		if it.Archived {
			continue
		}
		resp.ActiveItems++
		resp.ByCategory[it.Category]++
		m := store.MonthlyCost(it)
		resp.MonthlyCost += m
		resp.YearlyCost += store.YearlyCost(it)
		if m > 0 {
			resp.CostByCategory[it.Category] += m
		}
		switch store.StatusFor(it, today) {
		case store.StatusOverdue:
			resp.Overdue++
		case store.StatusDueSoon:
			resp.DueSoon++
		}
		next, _ := store.NextOccurrence(it, today)
		if d := store.DaysUntil(next, today); d >= 0 && d <= 30 {
			resp.Next30Days++
		}
	}
	resp.MonthlyCost = round2(resp.MonthlyCost)
	resp.YearlyCost = round2(resp.YearlyCost)
	for k, v := range resp.CostByCategory {
		resp.CostByCategory[k] = round2(v)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Settings())
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var set store.Settings
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&set); err != nil {
		writeErr(w, http.StatusBadRequest, errors.New("invalid request body: "+err.Error()))
		return
	}
	saved, err := s.store.UpdateSettings(set)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleSample(w http.ResponseWriter, _ *http.Request) {
	n, err := s.store.AddAll(sampleItems(s.now().UTC()))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"added": n})
}
