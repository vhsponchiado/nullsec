# Contributing to Nullsec

Thanks for taking an interest in Nullsec! This is a young project, so please
read through this guide before opening a pull request — it'll save both of
us a round-trip.

## Code of conduct

Please read the [Code of Conduct](CODE_OF_CONDUCT.md) before participating.

## Reporting bugs and requesting features

Use [issues](https://github.com/nullsec-proxy/nullsec/issues) for bugs and
feature requests. Include enough to reproduce: OS, Go/Node versions, and the
exact steps that triggered the problem.

## Local development setup

You need Go 1.26+, Node 20+, and (optionally) a local Chrome/Chromium install
if you're touching `internal/usecase/chrome` or `internal/adapter/chromedp`.

```bash
# Terminal 1 — backend: proxy on :8080, REST API + WebSocket on :8081
cd backend
go run ./cmd/server

# Terminal 2 — frontend: http://localhost:5173
cd frontend
npm install   # first time only
npm run dev
```

Before opening a PR, run the same checks CI runs:

```bash
# Backend
cd backend
gofmt -l .              # must print nothing
go vet ./...
go test ./...
golangci-lint run       # see .golangci.yml; `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`

# Frontend
cd frontend
npx tsc --noEmit -p tsconfig.app.json
npx oxlint src
npm run build
```

## Project layout

The backend follows Clean Architecture: dependencies only point inward
(`entity` ← `usecase` ← `adapter`/`infra` ← `cmd`). Concretely:

- `backend/cmd/server` — the composition root (`main.go`) and CLI
  subcommands (`cert`, `chrome`). The only place allowed to import every
  layer and wire concrete implementations into the interfaces each use case
  declares.
- `backend/internal/entity` — core domain types with zero framework
  dependencies: `Request`/`RequestSummary`, `Severity`, `ScanStatus`,
  `UpstreamProxyConfig`, plus the handful of ports shared by more than one
  use case (`HistoryRepository`, `Broadcaster`, event-name constants).
- `backend/internal/usecase/*` — one package per capability, framework-agnostic:
  - `proxy` — intercept queues (`Interceptor`, `ResponseInterceptor`) and
    `Forwarder`, the "run a request through intercept, round-trip it, record
    it" pipeline shared by plain HTTP and decrypted HTTPS.
  - `sqli`, `xss`, `wpcheck`, `passivescan` — the active/passive scanners.
    See "Adding a new scanner tool" below.
  - `intruder`, `portscan`, `subdomain`, `spider` — the attack/recon engines,
    each a background-job `Manager`.
  - `chrome`, `certificate`, `settings`, `repeater`, `decoder` — the
    remaining tools, each thin wrappers around a port implemented in
    `adapter`/`infra`.
  - Any use case needing something from outside declares a small interface
    locally (e.g. `portscan.Dialer`, `chrome.Launcher`) rather than
    importing the concrete adapter/infra package — that's what keeps this
    layer testable and swappable.
- `backend/internal/adapter/*` — translates use cases to/from the outside
  world: `httpapi` (REST controllers — keep these thin: decode, call the
  use case, write the response), `ws` (WebSocket `Broadcaster`),
  `sqliterepo` (SQLite `HistoryRepository`), `chromedp` (chromedp-backed
  `Launcher`).
- `backend/internal/infra/*` — the lowest-level, most volatile code:
  `mitm` (the actual CONNECT/TLS engine), `ca` (CA keypair generation),
  `upstream` (SOCKS5/HTTP dialing).
- `frontend/src/features/*` — one folder per tool, mirroring the sidebar.
  Most non-trivial tools pair their main component with a
  `<feature>-guide.tsx` explainer panel — follow that convention for new
  tools.
- `frontend/src/components` — shared, feature-agnostic UI. Nothing in here
  should import from `src/features/*`.

## Adding a new scanner tool

The active scanners (SQLi, XSS, WordPress) and the passive scanner all follow
the same shape: `entity.Severity` for findings, a `Finding`/`Report` struct,
a `Run(ctx, target) (Report, error)` entrypoint, and a thin `adapter/httpapi`
handler that decodes the request and calls it. Look at
`internal/usecase/sqli` as the reference implementation before starting a
new one — most of the request/response plumbing (injection-point
enumeration, request cloning) is meant to be reused, not re-derived.

On the frontend, a new scanner typically needs: a `features/<name>/<name>.tsx`
page, a `features/<name>/<name>-guide.tsx` explainer, a nav entry in
`src/components/app-sidebar.tsx`, and a route in `src/App.tsx`.

## Pull requests

- Keep PRs focused — a bug fix and a refactor in the same PR are harder to
  review and to revert independently.
- Add tests for new logic, especially in the scanner packages
  (`internal/usecase/sqli`, `internal/usecase/xss`, etc.) — they run against
  arbitrary user-supplied targets, and a bad regex or parsing change there
  is the kind of thing that's easy to get subtly wrong.
- For anything that changes how interception, certificate generation, or
  the proxy's connection handling works, please explain the reasoning in
  the PR description — that code trades off correctness against security
  properties (see the doc comments in `internal/infra/ca` and
  `internal/infra/mitm`), and the trade-off should be spelled out, not just
  the diff.
