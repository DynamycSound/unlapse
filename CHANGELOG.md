# Changelog

All notable changes to unlapse are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [1.0.0] — 2026-06-11

Initial release.

### Added
- Track anything that expires or renews: subscriptions, warranties, insurance,
  documents, domains, memberships, and a free-form "other" category.
- Recurrence engine with weekly / monthly / quarterly / yearly / every-N-days
  cycles, month-end clamping, and proper overdue handling for one-time items.
- Dashboard: needs-attention list, 90-day outlook, monthly and yearly spend
  totals, per-category cost bars and counts.
- Per-item reminder lead times with overdue / due-soon / upcoming statuses.
- Live iCalendar feed (`/calendar.ics`) with per-item alarms and RRULEs —
  subscribe from Google Calendar, Apple Calendar, Outlook, or Thunderbird.
- CSV export and all-or-nothing CSV import.
- Search, category filters, archive, dark and light themes, sample data.
- Single-binary distribution for Windows, macOS (Intel & Apple Silicon), and
  Linux (amd64 & arm64), plus a `scratch`-based Dockerfile.
- Data stored in one human-readable JSON file, written atomically.
- Zero runtime dependencies, zero network calls, zero telemetry.

[1.0.0]: https://github.com/DynamycSound/unlapse/releases/tag/v1.0.0
