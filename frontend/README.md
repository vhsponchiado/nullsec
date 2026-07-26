# Nullsec — frontend

React + Vite + TypeScript UI for [Nullsec](../README.md), a Burp Suite-style
HTTP/HTTPS intercepting proxy. This is the web app you point your browser at
(default http://localhost:5173); the Go backend it talks to lives in
[`../backend`](../backend).

Built with React 19, Tailwind CSS 4, shadcn/ui (Radix primitives), and oxlint.

## Running

The backend must already be running (see the [root README](../README.md) for
that) — this app has nothing to render without it.

```bash
npm install   # first time only
npm run dev
```

Opens at http://localhost:5173.

## Configuration

The backend's REST API + WebSocket default to `http://localhost:8081`. If
you've moved it (e.g. via the backend's `API_PORT` env var), point this app
at it with a `.env.local`:

```
VITE_API_BASE=http://localhost:8081
VITE_WS_BASE=ws://localhost:8081
```

## Project layout

- `src/features/*` — one folder per tool, mirroring the sidebar (Proxy
  History, Intercept, Repeater, Intruder, the recon/scanner tools, Decoder,
  Certificate, Settings). Most non-trivial tools pair their main component
  with a `<feature>-guide.tsx` explainer panel — follow that convention if
  you add a new one.
- `src/components` — shared, feature-agnostic UI (request/response editors,
  the header/key-value table, shadcn primitives under `components/ui`).
  Nothing here should import from `src/features/*`.
- `src/lib` — the API client (`api.ts`), the WebSocket event hook (`ws.ts`),
  shared types (`types.ts`), and small parsing helpers (`headers.ts`,
  `http-status.ts`).

## Checks

```bash
npx tsc --noEmit -p tsconfig.app.json   # typecheck
npm run lint                            # oxlint
npm run build                           # production build
```

See [../CONTRIBUTING.md](../CONTRIBUTING.md) for the full contribution guide.
