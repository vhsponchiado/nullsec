# Nullsec Proxy

A Burp Suite-style HTTP/HTTPS intercepting proxy: backend in Go (proxy +
SQLite history + WebSocket) and frontend in Vite/React/shadcn-ui.

Current state: HTTP/HTTPS MITM proxy with its own CA, Intercept (requests
and responses), history, and Repeater are all working, alongside a growing
set of recon and scanning tools — Intruder (sniper/battering ram/pitchfork/
cluster bomb), Spider, Passive Scan, Port Scanner, Subdomain Finder,
WordPress Check, SQL Injection and XSS scanners, a Burp-style Decoder, and
an upstream-proxy setting (SOCKS5/HTTP) so any of the above can be routed
through infrastructure you're authorized to use.

Licensed under [Apache-2.0](LICENSE). Contributions welcome — see
[CONTRIBUTING.md](CONTRIBUTING.md).

## Running locally

**1. Backend**

```bash
cd backend
go run ./cmd/server
```

Starts the HTTP/HTTPS proxy on port `8080` and the REST API + WebSocket on
port `8081`. Persists to `backend/nullsec.db` (SQLite, created
automatically). On first run it also generates its own CA at
`~/.nullsec/ca-cert.pem` and `~/.nullsec/ca-key.pem`.
Env vars: `PROXY_PORT`, `API_PORT`, `DB_PATH`, `CA_CERT_PATH`, `CA_KEY_PATH`.

**Trusting the CA (needed for HTTPS without certificate errors)**

```bash
# download the certificate
curl http://localhost:8081/api/ca/cert -o nullsec-ca-cert.pem

# install into the system trust store
go run ./cmd/server cert install
```

`cert install` uses [smallstep/truststore](https://github.com/smallstep/truststore)
(the same lib Hetty uses) to install it automatically — no need to import it
manually into every browser. May prompt for your password (for the system
trust store). To undo it: `go run ./cmd/server cert uninstall`.

> Without installing the CA, HTTPS via the proxy still works (interception
> happens normally), but the browser will show an untrusted-certificate
> warning on every site, since certificates are signed by the Nullsec CA
> rather than a public CA.

**2. Frontend**

```bash
cd frontend
npm install   # first time only
npm run dev
```

Opens at http://localhost:5173.

**3. Generate traffic through the proxy**

With `curl`:

```bash
curl -x http://localhost:8080 http://example.com
```

Through Chrome, in a throwaway profile (recommended — the same technique
Burp Suite and Hetty's `--chrome` use: proxy + ignored certificate errors,
no need to trust any CA). This drives Chrome directly via
[chromedp](https://github.com/chromedp/chromedp) (see `internal/chrome`)
rather than a shell script, so the throwaway profile is always cleaned up
once the window closes, whether it was closed by hand or stopped from the
command below:

```bash
go run ./cmd/server chrome https://example.com
```

Opens a new, isolated window (temporary profile, doesn't touch your real
Chrome), already configured to use the proxy and ignore certificate
warnings. Close the window (or Ctrl+C the command) when you're done — the
throwaway profile keeps nothing saved between runs.

By default it also sideloads your installed extensions from your real
default Chrome profile (`~/.config/google-chrome/Default`), so ad blockers,
a password manager, etc. are still available — only the extensions come
along, not cookies/history/logins. Chrome will show a "disable developer
mode extensions" nag banner for these — that's expected, just dismiss it.
Pass `--load-extensions=false` for a fully blank profile instead, or
`--real-profile=/path/to/profile` if your real profile isn't at the default
location.

**4. Using the UI**

Navigate via the icon sidebar on the left:

- **Proxy History**: everything that went through the proxy, with search by
  host and a request/response detail view (Query Params/Headers/Body tabs).
  The trash icon clears the history.
- **Intercept**: **Requests** and **Responses** sub-tabs — flip the
  corresponding toggle to hold requests before forwarding, or responses
  before returning them to the client (like Burp's "Intercept server
  responses"). Edit and Forward/Drop.
- **Repeater**: send a history entry ("Send to Repeater") to resend it as
  many times as you want, editing freely.
- **Intruder**: mark payload positions in a request (`§…§`), pick an attack
  type (sniper, battering ram, pitchfork, cluster bomb), fill in payload
  sets (custom list, built-in common-passwords wordlist, or a number
  range), and fire — results stream in live, sortable, with an optional
  grep-match column.
- **Spider**: crawls a target from a seed URL up to a configurable depth/page
  count, discovering pages and forms.
- **Passive Scan**: analyzes traffic already captured in Proxy History for
  missing security headers, cookie flags, permissive CORS, cleartext
  credentials, and similar issues — no extra requests made.
- **Port Scanner**: an nmap-style TCP connect scan (top 100/1000 ports or a
  custom range) with adjustable speed and per-port banner grabbing.
- **Subdomain Finder**: combines passive discovery (crt.sh Certificate
  Transparency logs) with active DNS brute-forcing from a built-in
  frequency-ranked wordlist.
- **WordPress Check**: point it at a target and it fingerprints known
  WordPress endpoints (admin panel, REST API, user enumeration, exposed
  backups, etc.) and reports what it found.
- **SQL Injection** / **XSS**: automatic or manual scanning against a
  request's parameters, using built-in payload wordlists (SQLi:
  error-based, boolean-based, time-based; XSS: reflected-payload signal
  detection across HTML/attribute/JS/URL contexts).
- **Decoder**: a Burp-style chained encode/decode tool (Base64/32, hex, URL,
  HTML entities, gzip, MD5/SHA-1/SHA-256/SHA-512) with a "smart decode" that
  auto-detects and peels off layers.
- **Settings**: route Repeater, Intruder, the Port Scanner, and the
  Subdomain Finder's crt.sh lookup through an upstream SOCKS5 or HTTP proxy.
- **Certificate**: download the CA, regenerate it, or see the automatic
  install commands (`cert install`) and the throwaway Chrome option.

Right-click any row in Proxy History or Intercept to send it straight to
Repeater, Intruder, SQL Injection, or XSS — headers (cookies, auth, etc.)
come along automatically since they're captured from the real request.

## Running with Docker

A single image builds and serves everything — proxy, API/WebSocket, and the
frontend's static build on the same origin, so no separate frontend
container or build-time API URL is needed.

**With Docker Compose** (simplest — reads `docker-compose.yml`):

```bash
docker compose up -d --build
```

Change the ports without touching any file:

```bash
PROXY_PORT=9090 API_PORT=9091 docker compose up -d
```

**Or plain `docker build`/`run`**, equivalent to the above:

```bash
docker build -t nullsec .
docker run -d --name nullsec \
  -p 8080:8080 -p 8081:8081 \
  -v nullsec-data:/data \
  nullsec
```

Open http://localhost:8081. The `nullsec-data` volume is where the CA
keypair and the SQLite history file (`/data/ca-cert.pem`, `/data/ca-key.pem`,
`/data/nullsec.db` by default) persist across restarts — without it, both
regenerate empty on every `docker run`, and you'd have to re-trust a new CA
each time.

Configure the container purely through env vars (same ones the binary reads
outside Docker — see `PROXY_PORT`/`API_PORT`/`DB_PATH`/`CA_CERT_PATH`/`CA_KEY_PATH`
above), e.g. `docker run -e PROXY_PORT=9090 -p 9090:9090 ...`.

One thing Docker can't do for you: **trusting the CA.** `cert install` (see
above) modifies the *host's* OS trust store, which isn't reachable from
inside a container. Download the cert from a running container (`curl
http://localhost:8081/api/ca/cert -o nullsec-ca-cert.pem`) and trust it
manually, or just accept the untrusted-certificate warning on HTTPS sites,
same as running without `cert install` at all.

For a throwaway-Chrome-with-the-proxy-preconfigured browsing session, run
the `chrome` CLI subcommand on your own machine (see "Installing a prebuilt
binary" below for `nullsec-server` with no Go toolchain needed) — a
real/visible browser window is inherently a host-side thing, not something
a container can spawn for you — pointing `--proxy-host`/`--proxy-port` at
wherever the container's proxy port is exposed:

```bash
nullsec-server chrome --proxy-host=<container-host> --proxy-port=8080 https://target.example
```

### Practice target (DVWA)

`docker-compose.yml` also has [DVWA](https://github.com/digininja/DVWA)
(`dvwa`), an intentionally vulnerable practice app — a safe target for the
main `nullsec` service above.

```bash
docker compose up -d dvwa
```

1. Open http://localhost:8888/setup.php and click "Create / Reset Database".
2. Log in at http://localhost:8888/login.php — `admin` / `password`.
3. **DVWA Security** (left nav) → set to **low**, so the scanners have
   something to actually find.
4. Point your browser's proxy settings at `127.0.0.1:8080` (nullsec).
5. Browse to **http://dvwa/** — not `localhost:8888`. From inside nullsec's
   container, `dvwa` is the name that resolves (Docker Compose's own network
   DNS); `localhost:8888` is only reachable from your host, not from inside
   the proxy's container.
6. Attack away — Intruder, SQL Injection, XSS, Spider, whatever you're
   practicing. Everything lands in nullsec's normal Proxy History at
   http://localhost:8081.

## Installing a prebuilt binary

No Go toolchain needed — grab a release binary and run it directly. Handy
for the `chrome` subcommand above, or for running the whole server natively
instead of via Docker.

```bash
curl -fsSL https://raw.githubusercontent.com/nullsec-proxy/nullsec/main/install.sh | sh
```

Installs `nullsec-server` to `/usr/local/bin` (set `NULLSEC_INSTALL_DIR` to
install elsewhere). Linux and macOS, amd64 and arm64. On Windows, download
the `.exe` directly from the
[latest release](https://github.com/nullsec-proxy/nullsec/releases/latest).

Every push of a `v*` tag builds and publishes these via
[`.github/workflows/release.yml`](.github/workflows/release.yml), alongside
a `checksums.txt` `install.sh` verifies against before installing.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for local dev setup, the checks CI
runs, and the conventions for adding a new scanner tool.
