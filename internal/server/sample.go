package server

import (
	"time"

	"github.com/DynamycSound/unlapse/internal/store"
)

// sampleItems returns realistic demo data anchored around today, so the
// dashboard immediately shows overdue, due-soon, and upcoming buckets.
func sampleItems(today time.Time) []store.Item {
	d := func(days int) string { return today.AddDate(0, 0, days).Format("2006-01-02") }
	return []store.Item{
		{Name: "Video streaming", Category: "subscription", Date: d(3), Cycle: "monthly", Cost: 15.49, RemindDays: 5, Notes: "Family plan — check if anyone still uses it"},
		{Name: "Music streaming", Category: "subscription", Date: d(11), Cycle: "monthly", Cost: 10.99, RemindDays: 5},
		{Name: "Cloud photo storage", Category: "subscription", Date: d(19), Cycle: "monthly", Cost: 2.99, RemindDays: 7, Notes: "200 GB tier"},
		{Name: "Password manager", Category: "subscription", Date: d(54), Cycle: "yearly", Cost: 36, RemindDays: 21},
		{Name: "VPN service", Category: "subscription", Date: d(-2), Cycle: "yearly", Cost: 59.88, RemindDays: 14, Notes: "Renewal price doubles after intro year — renegotiate or switch"},
		{Name: "Gym membership", Category: "membership", Date: d(8), Cycle: "monthly", Cost: 39, RemindDays: 7},
		{Name: "Costco membership", Category: "membership", Date: d(132), Cycle: "yearly", Cost: 65, RemindDays: 30},
		{Name: "Car insurance", Category: "insurance", Date: d(26), Cycle: "quarterly", Cost: 287.5, RemindDays: 21, Notes: "Shop around before renewal — last quote beat it by $40"},
		{Name: "Home contents insurance", Category: "insurance", Date: d(97), Cycle: "yearly", Cost: 312, RemindDays: 30},
		{Name: "Passport", Category: "document", Date: d(176), Cycle: "none", RemindDays: 180, Notes: "Many countries require 6 months validity — renew early"},
		{Name: "Driver's license", Category: "document", Date: d(412), Cycle: "none", RemindDays: 60},
		{Name: "Laptop warranty", Category: "warranty", Date: d(64), Cycle: "none", RemindDays: 30, Notes: "Extended coverage — claim the flickering screen before it ends"},
		{Name: "Washing machine warranty", Category: "warranty", Date: d(-9), Cycle: "none", RemindDays: 30},
		{Name: "Personal domain", Category: "domain", Date: d(41), Cycle: "yearly", Cost: 12.99, RemindDays: 30, URL: "https://example.com"},
		{Name: "Smoke detector batteries", Category: "other", Date: d(143), Cycle: "custom", CycleDays: 182, RemindDays: 7, Notes: "Replace every 6 months"},
	}
}
