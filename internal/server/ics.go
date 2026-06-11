package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/DynamycSound/unlapse/internal/store"
)

// icsEscape escapes text per RFC 5545 §3.3.11.
func icsEscape(s string) string {
	r := strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\r\n", "\\n", "\n", "\\n")
	return r.Replace(s)
}

// icsRRule maps an item's cycle to an RFC 5545 recurrence rule ("" for one-time).
func icsRRule(it store.Item) string {
	switch it.Cycle {
	case "weekly":
		return "FREQ=WEEKLY"
	case "monthly":
		return "FREQ=MONTHLY"
	case "quarterly":
		return "FREQ=MONTHLY;INTERVAL=3"
	case "yearly":
		return "FREQ=YEARLY"
	case "custom":
		if it.CycleDays > 0 {
			return fmt.Sprintf("FREQ=DAILY;INTERVAL=%d", it.CycleDays)
		}
	}
	return ""
}

// handleICS serves a live iCalendar feed of every active item's next due date,
// with a reminder alarm at each item's lead time. Subscribe to it from any
// calendar app to get renewal reminders without unlapse running a notifier.
func (s *Server) handleICS(w http.ResponseWriter, _ *http.Request) {
	today := s.now().UTC()
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//unlapse//unlapse " + Version + "//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("X-WR-CALNAME:unlapse renewals\r\n")

	stamp := today.Format("20060102T150405Z")
	for _, it := range s.store.Items() {
		if it.Archived {
			continue
		}
		next, _ := store.NextOccurrence(it, today)
		date := next.Format("20060102")
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString("UID:" + it.ID + "@unlapse\r\n")
		b.WriteString("DTSTAMP:" + stamp + "\r\n")
		b.WriteString("DTSTART;VALUE=DATE:" + date + "\r\n")
		b.WriteString("SUMMARY:" + icsEscape(eventTitle(it)) + "\r\n")
		if it.Notes != "" {
			b.WriteString("DESCRIPTION:" + icsEscape(it.Notes) + "\r\n")
		}
		if rrule := icsRRule(it); rrule != "" {
			b.WriteString("RRULE:" + rrule + "\r\n")
		}
		if it.RemindDays > 0 {
			b.WriteString("BEGIN:VALARM\r\n")
			b.WriteString("ACTION:DISPLAY\r\n")
			b.WriteString("DESCRIPTION:" + icsEscape(eventTitle(it)) + "\r\n")
			b.WriteString(fmt.Sprintf("TRIGGER:-P%dD\r\n", it.RemindDays))
			b.WriteString("END:VALARM\r\n")
		}
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="unlapse.ics"`)
	_, _ = w.Write([]byte(b.String()))
}

func eventTitle(it store.Item) string {
	verb := "expires"
	if it.Cycle != "none" && it.Cycle != "" {
		verb = "renews"
	}
	return fmt.Sprintf("%s %s (unlapse)", it.Name, verb)
}
