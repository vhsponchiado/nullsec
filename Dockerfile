# syntax=docker/dockerfile:1

# Builds a single image serving everything: the MITM proxy, the REST/WS API,
# and the frontend's static build (same-origin, via STATIC_DIR — see
# backend/cmd/server/main.go) so no build-time API/WS URL needs to be baked
# into the frontend bundle. Configure at `docker run` time purely with env
# vars; see README for the full list.

FROM node:22-alpine AS frontend-builder
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend-builder
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# CGO_ENABLED=0: internal/adapter/sqliterepo uses modernc.org/sqlite, a
# pure-Go driver, so the binary needs no C toolchain or shared libs at
# runtime — that's what lets the final stage be a bare, minimal image.
RUN CGO_ENABLED=0 go build -trimpath -o /out/nullsec-server ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=backend-builder /out/nullsec-server /usr/local/bin/nullsec-server
COPY --from=frontend-builder /src/frontend/dist /app/frontend/dist

# CA_KEY_PATH is a filesystem path, not a secret value — the key material
# itself lives on the /data volume below, generated at first run.
ENV STATIC_DIR=/app/frontend/dist \
    PROXY_PORT=8080 \
    API_PORT=8081 \
    DB_PATH=/data/nullsec.db \
    CA_CERT_PATH=/data/ca-cert.pem \
    CA_KEY_PATH=/data/ca-key.pem

# CA material and captured proxy history both live here — mount a volume so
# they survive container restarts instead of regenerating (and losing
# history) every run.
VOLUME /data
EXPOSE 8080 8081

ENTRYPOINT ["nullsec-server"]
