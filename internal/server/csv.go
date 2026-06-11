package server

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/DynamycSound/unlapse/internal/store"
)

var csvHeader = []string{
	"name", "category", "date", "cycle", "cycleDays", "cost", "remindDays", "url", "notes", "archived",
}

func (s *Server) handleExportCSV(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="unlapse-export.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write(csvHeader)
	for _, it := range s.store.Items() {
		_ = cw.Write([]string{
			it.Name,
			it.Category,
			it.Date,
			it.Cycle,
			strconv.Itoa(it.CycleDays),
			strconv.FormatFloat(it.Cost, 'f', -1, 64),
			strconv.Itoa(it.RemindDays),
			it.URL,
			it.Notes,
			strconv.FormatBool(it.Archived),
		})
	}
	cw.Flush()
}

// handleImportCSV accepts a CSV body (same columns as the export) and adds
// every row as a new item. The whole file is validated before anything is
// saved, so an import either fully succeeds or changes nothing.
func (s *Server) handleImportCSV(w http.ResponseWriter, r *http.Request) {
	cr := csv.NewReader(http.MaxBytesReader(w, r.Body, 4<<20))
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid CSV: %w", err))
		return
	}
	if len(rows) < 2 {
		writeErr(w, http.StatusBadRequest, errors.New("CSV has no data rows"))
		return
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[strings.TrimSpace(strings.ToLower(h))] = i
	}
	if _, ok := col["name"]; !ok {
		writeErr(w, http.StatusBadRequest, errors.New(`CSV must have a "name" column`))
		return
	}
	field := func(row []string, name string) string {
		i, ok := col[strings.ToLower(name)]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	defaults := s.store.Settings()
	items := make([]store.Item, 0, len(rows)-1)
	for n, row := range rows[1:] {
		it := store.Item{
			Name:     field(row, "name"),
			Category: field(row, "category"),
			Date:     field(row, "date"),
			Cycle:    field(row, "cycle"),
			URL:      field(row, "url"),
			Notes:    field(row, "notes"),
		}
		if it.Category == "" {
			it.Category = "other"
		}
		if it.Cycle == "" {
			it.Cycle = "none"
		}
		if v := field(row, "cycledays"); v != "" {
			if it.CycleDays, err = strconv.Atoi(v); err != nil {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("row %d: bad cycleDays %q", n+2, v))
				return
			}
		}
		if v := field(row, "cost"); v != "" {
			if it.Cost, err = strconv.ParseFloat(v, 64); err != nil {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("row %d: bad cost %q", n+2, v))
				return
			}
		}
		it.RemindDays = defaults.DefaultRemindDays
		if v := field(row, "reminddays"); v != "" {
			if it.RemindDays, err = strconv.Atoi(v); err != nil {
				writeErr(w, http.StatusBadRequest, fmt.Errorf("row %d: bad remindDays %q", n+2, v))
				return
			}
		}
		if v := field(row, "archived"); v != "" {
			it.Archived = strings.EqualFold(v, "true") || v == "1"
		}
		if err := it.Validate(); err != nil {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("row %d: %w", n+2, err))
			return
		}
		items = append(items, it)
	}

	added, err := s.store.AddAll(items)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"added": added})
}
