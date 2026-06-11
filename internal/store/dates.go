package store

import (
	"time"
)

// ParseDate parses a YYYY-MM-DD string as a UTC midnight time.
func ParseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// addMonthsClamped adds months while clamping the day to the target month's
// length, so Jan 31 + 1 month = Feb 28/29 rather than Go's normalized Mar 2/3.
func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).AddDate(0, months, 0)
	lastDay := first.AddDate(0, 1, -1).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, time.UTC)
}

// advance returns the occurrence after t for the item's cycle.
func advance(it Item, t time.Time) time.Time {
	switch it.Cycle {
	case "weekly":
		return t.AddDate(0, 0, 7)
	case "monthly":
		return addMonthsClamped(t, 1)
	case "quarterly":
		return addMonthsClamped(t, 3)
	case "yearly":
		return addMonthsClamped(t, 12)
	case "custom":
		days := it.CycleDays
		if days < 1 {
			days = 1
		}
		return t.AddDate(0, 0, days)
	default: // "none": never advances
		return t
	}
}

// NextOccurrence returns the item's next due date on or after today.
// One-time items keep their anchor date even when past (they are overdue);
// recurring items roll forward from the anchor. The bool reports whether the
// returned date is in the past (overdue), which can only happen for one-time
// items or invalid dates.
func NextOccurrence(it Item, today time.Time) (time.Time, bool) {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	anchor, err := ParseDate(it.Date)
	if err != nil {
		return today, false
	}
	if it.Cycle == "none" || it.Cycle == "" {
		return anchor, anchor.Before(today)
	}
	next := anchor
	for i := 0; next.Before(today) && i < 5000; i++ {
		next = advance(it, next)
	}
	return next, false
}

// DaysUntil returns whole days from today to date (negative if past).
func DaysUntil(date, today time.Time) int {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	return int(date.Sub(today).Hours() / 24)
}

// cyclesPerYear converts a billing cycle to occurrences per year.
func cyclesPerYear(it Item) float64 {
	switch it.Cycle {
	case "weekly":
		return 365.25 / 7
	case "monthly":
		return 12
	case "quarterly":
		return 4
	case "yearly":
		return 1
	case "custom":
		if it.CycleDays > 0 {
			return 365.25 / float64(it.CycleDays)
		}
	}
	return 0
}

// YearlyCost is the item's recurring cost per year (0 for one-time items).
func YearlyCost(it Item) float64 {
	return it.Cost * cyclesPerYear(it)
}

// MonthlyCost is the item's recurring cost per month (0 for one-time items).
func MonthlyCost(it Item) float64 {
	return YearlyCost(it) / 12
}

// Status buckets an item by urgency for the given day.
type Status string

const (
	StatusOverdue  Status = "overdue"
	StatusDueSoon  Status = "due-soon"
	StatusUpcoming Status = "upcoming"
	StatusOK       Status = "ok"
)

// StatusFor classifies: overdue (past), due-soon (within remind window),
// upcoming (within 30 days beyond the window), ok otherwise.
func StatusFor(it Item, today time.Time) Status {
	next, overdue := NextOccurrence(it, today)
	if overdue {
		return StatusOverdue
	}
	days := DaysUntil(next, today)
	if days <= it.RemindDays {
		return StatusDueSoon
	}
	if days <= 30 {
		return StatusUpcoming
	}
	return StatusOK
}
