# Contributing to unlapse

Thanks for wanting to help! unlapse is intentionally small and dependency-free,
and contributions that keep it that way are very welcome.

## Ground rules

- **Zero runtime dependencies.** The Go module has no third-party imports and
  the frontend has no framework and no build step. Please keep it that way —
  a PR that adds a dependency needs a very strong reason.
- **Local-first, always.** No feature may phone home, require an account, or
  send user data anywhere. No telemetry, ever — this is a headline feature.
- **Everything ships working.** New features need tests for the logic and must
  work in both themes.

## Getting started

```bash
git clone https://github.com/DynamycSound/unlapse
cd unlapse
go run ./cmd/unlapse
# open http://127.0.0.1:8275
```

The frontend lives in `internal/server/webdist/` and is embedded into the
binary at compile time — edit the files, rebuild (or `go run`) and refresh.

## Before opening a PR

```bash
gofmt -l .        # must print nothing
go vet ./...
go test -race ./...
```

- Keep PRs focused: one feature or fix per PR.
- Describe *why*, not just *what*, in the PR description.
- For UI changes, attach a screenshot (dark theme at minimum).

## Reporting bugs / requesting features

Use the issue templates. For bugs, include your OS, how you run unlapse
(binary or Docker), and steps to reproduce.

## Code of conduct

Be kind. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
