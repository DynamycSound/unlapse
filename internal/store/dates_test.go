package store

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNextOccurrenceOneTime(t *testing.T) {
	it := Item{Date: "2026-08-01", Cycle: "none"}

	next, overdue := NextOccurrence(it, day("2026-06-11"))
	if next != day("2026-08-01") || overdue {
		t.Fatalf("future one-time: got %v overdue=%v", next, overdue)
	}

	next, overdue = NextOccurrence(it, day("2026-09-01"))
	if next != day("2026-08-01") || !overdue {
		t.Fatalf("past one-time should stay anchored and be overdue: got %v overdue=%v", next, overdue)
	}
}

func TestNextOccurrenceRecurring(t *testing.T) {
	cases := []struct {
		name  string
		item  Item
		today string
		want  string
	}{
		{"monthly rolls forward", Item{Date: "2026-01-15", Cycle: "monthly"}, "2026-06-11", "2026-06-15"},
		{"monthly on anchor day", Item{Date: "2026-06-11", Cycle: "monthly"}, "2026-06-11", "2026-06-11"},
		{"yearly", Item{Date: "2024-03-01", Cycle: "yearly"}, "2026-06-11", "2027-03-01"},
		{"weekly", Item{Date: "2026-06-01", Cycle: "weekly"}, "2026-06-11", "2026-06-15"},
		{"quarterly", Item{Date: "2026-01-10", Cycle: "quarterly"}, "2026-06-11", "2026-07-10"},
		{"custom 182d", Item{Date: "2026-01-01", Cycle: "custom", CycleDays: 182}, "2026-06-11", "2026-07-02"},
		{"month-end clamps", Item{Date: "2026-01-31", Cycle: "monthly"}, "2026-02-01", "2026-02-28"},
		{"leap year clamps to 29", Item{Date: "2028-01-31", Cycle: "monthly"}, "2028-02-01", "2028-02-29"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next, overdue := NextOccurrence(c.item, day(c.today))
			if overdue {
				t.Fatalf("recurring items are never overdue")
			}
			if got := next.Format("2006-01-02"); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestCosts(t *testing.T) {
	monthly := Item{Cost: 12, Cycle: "monthly"}
	if got := YearlyCost(monthly); got != 144 {
		t.Fatalf("monthly yearly cost: got %v", got)
	}
	yearly := Item{Cost: 120, Cycle: "yearly"}
	if got := MonthlyCost(yearly); got != 10 {
		t.Fatalf("yearly monthly cost: got %v", got)
	}
	oneTime := Item{Cost: 500, Cycle: "none"}
	if YearlyCost(oneTime) != 0 || MonthlyCost(oneTime) != 0 {
		t.Fatalf("one-time items must not contribute to recurring spend")
	}
	weekly := Item{Cost: 7, Cycle: "weekly"}
	want := 7 * 365.25 / 7
	if got := YearlyCost(weekly); got != want {
		t.Fatalf("weekly yearly cost: got %v want %v", got, want)
	}
	custom := Item{Cost: 10, Cycle: "custom", CycleDays: 73}
	if got := YearlyCost(custom); got < 50 || got > 50.1 {
		t.Fatalf("custom yearly cost: got %v, want ~50.03", got)
	}
}

func TestStatusFor(t *testing.T) {
	today := day("2026-06-11")
	cases := []struct {
		name string
		item Item
		want Status
	}{
		{"overdue one-time", Item{Date: "2026-06-01", Cycle: "none", RemindDays: 7}, StatusOverdue},
		{"inside remind window", Item{Date: "2026-06-15", Cycle: "none", RemindDays: 7}, StatusDueSoon},
		{"due today", Item{Date: "2026-06-11", Cycle: "none", RemindDays: 0}, StatusDueSoon},
		{"upcoming within 30", Item{Date: "2026-07-05", Cycle: "none", RemindDays: 7}, StatusUpcoming},
		{"far away", Item{Date: "2026-12-01", Cycle: "none", RemindDays: 7}, StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StatusFor(c.item, today); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	good := Item{Name: "Passport", Category: "document", Date: "2027-01-01", Cycle: "none", RemindDays: 90}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid item rejected: %v", err)
	}
	bad := []Item{
		{Name: "", Category: "other", Date: "2027-01-01"},
		{Name: "x", Category: "nope", Date: "2027-01-01"},
		{Name: "x", Category: "other", Date: "01/01/2027"},
		{Name: "x", Category: "other", Date: "2027-01-01", Cycle: "fortnightly"},
		{Name: "x", Category: "other", Date: "2027-01-01", Cycle: "custom", CycleDays: 0},
		{Name: "x", Category: "other", Date: "2027-01-01", Cost: -1},
		{Name: "x", Category: "other", Date: "2027-01-01", RemindDays: 9999},
	}
	for i, it := range bad {
		if err := it.Validate(); err == nil {
			t.Fatalf("bad item %d accepted", i)
		}
	}
}
