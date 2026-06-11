package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DynamycSound/unlapse/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(st)
	s.now = func() time.Time { return time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC) }
	return s
}

func do(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestItemLifecycleViaAPI(t *testing.T) {
	s := testServer(t)

	rec := do(t, s, "POST", "/api/items", `{"name":"Car insurance","category":"insurance","date":"2026-06-20","cycle":"quarterly","cost":300,"remindDays":14}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created struct {
		ID          string  `json:"id"`
		Status      string  `json:"status"`
		DaysLeft    int     `json:"daysLeft"`
		MonthlyCost float64 `json:"monthlyCost"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "due-soon" || created.DaysLeft != 9 || created.MonthlyCost != 100 {
		t.Fatalf("computed fields wrong: %+v", created)
	}

	rec = do(t, s, "GET", "/api/items", "")
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list: want 1 item, got %d", len(list))
	}

	rec = do(t, s, "PUT", "/api/items/"+created.ID, `{"name":"Car insurance","category":"insurance","date":"2026-06-20","cycle":"quarterly","cost":250,"remindDays":14,"archived":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	rec = do(t, s, "GET", "/api/items", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatal("archived item should be hidden by default")
	}
	rec = do(t, s, "GET", "/api/items?includeArchived=1", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatal("includeArchived=1 should show archived item")
	}

	rec = do(t, s, "DELETE", "/api/items/"+created.ID, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	rec = do(t, s, "DELETE", "/api/items/"+created.ID, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing: want 404, got %d", rec.Code)
	}
}

func TestValidationErrorsViaAPI(t *testing.T) {
	s := testServer(t)
	bad := []string{
		`{"name":"","category":"other","date":"2026-07-01"}`,
		`{"name":"x","category":"bogus","date":"2026-07-01"}`,
		`{"name":"x","category":"other","date":"not-a-date"}`,
		`{"name":"x","category":"other","date":"2026-07-01","unknownField":1}`,
		`not json at all`,
	}
	for i, body := range bad {
		if rec := do(t, s, "POST", "/api/items", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("case %d: want 400, got %d (%s)", i, rec.Code, rec.Body)
		}
	}
}

func TestStats(t *testing.T) {
	s := testServer(t)
	do(t, s, "POST", "/api/items", `{"name":"Sub","category":"subscription","date":"2026-06-15","cycle":"monthly","cost":12,"remindDays":7}`)
	do(t, s, "POST", "/api/items", `{"name":"Old warranty","category":"warranty","date":"2026-06-01","cycle":"none","remindDays":7}`)

	rec := do(t, s, "GET", "/api/stats", "")
	var st statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.ActiveItems != 2 || st.MonthlyCost != 12 || st.YearlyCost != 144 {
		t.Fatalf("stats totals wrong: %+v", st)
	}
	if st.Overdue != 1 || st.DueSoon != 1 {
		t.Fatalf("stats buckets wrong: %+v", st)
	}
	if st.ByCategory["subscription"] != 1 || st.CostByCategory["subscription"] != 12 {
		t.Fatalf("category maps wrong: %+v", st)
	}
}

func TestCSVRoundTrip(t *testing.T) {
	s := testServer(t)
	do(t, s, "POST", "/api/items", `{"name":"Domain, personal","category":"domain","date":"2026-09-01","cycle":"yearly","cost":12.99,"remindDays":30,"notes":"has, comma"}`)

	rec := do(t, s, "GET", "/api/export.csv", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("export: %d", rec.Code)
	}
	csvBody := rec.Body.String()
	if !strings.Contains(csvBody, `"Domain, personal"`) {
		t.Fatalf("export missing quoted name: %s", csvBody)
	}

	s2 := testServer(t)
	rec = do(t, s2, "POST", "/api/import", csvBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body)
	}
	rec = do(t, s2, "GET", "/api/items", "")
	var list []store.Item
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Name != "Domain, personal" || list[0].Cost != 12.99 || list[0].Notes != "has, comma" {
		t.Fatalf("round trip lost data: %+v", list)
	}
}

func TestImportRejectsBadRows(t *testing.T) {
	s := testServer(t)
	csvBody := "name,category,date\ngood,other,2026-07-01\nbad,other,not-a-date\n"
	rec := do(t, s, "POST", "/api/import", csvBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	rec = do(t, s, "GET", "/api/items", "")
	var list []store.Item
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatal("failed import must not partially apply")
	}
}

func TestICSFeed(t *testing.T) {
	s := testServer(t)
	do(t, s, "POST", "/api/items", `{"name":"Pass; port","category":"document","date":"2026-12-01","cycle":"none","remindDays":90}`)
	do(t, s, "POST", "/api/items", `{"name":"Gym","category":"membership","date":"2026-06-20","cycle":"monthly","cost":39,"remindDays":7}`)

	rec := do(t, s, "GET", "/calendar.ics", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("ics: %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"BEGIN:VCALENDAR",
		"SUMMARY:Pass\\; port expires (unlapse)",
		"SUMMARY:Gym renews (unlapse)",
		"RRULE:FREQ=MONTHLY",
		"TRIGGER:-P90D",
		"DTSTART;VALUE=DATE:20261201",
		"END:VCALENDAR",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("ics missing %q:\n%s", want, body)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/calendar") {
		t.Fatalf("content type: %s", ct)
	}
}

func TestSampleDataAndUIServed(t *testing.T) {
	s := testServer(t)
	rec := do(t, s, "POST", "/api/sample", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("sample: %d %s", rec.Code, rec.Body)
	}
	rec = do(t, s, "GET", "/api/items", "")
	var list []store.Item
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) < 10 {
		t.Fatalf("sample data too small: %d", len(list))
	}

	rec = do(t, s, "GET", "/", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "unlapse") {
		t.Fatalf("index not served: %d", rec.Code)
	}
	for _, asset := range []string{"/style.css", "/app.js", "/icon.svg"} {
		if rec := do(t, s, "GET", asset, ""); rec.Code != http.StatusOK {
			t.Fatalf("asset %s: %d", asset, rec.Code)
		}
	}
}
