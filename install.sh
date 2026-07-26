#!/usr/bin/env sh
# Downloads the latest nullsec-server release for your OS/arch, verifies its
# checksum, and installs it as `nullsec-server` on your PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/nullsec-proxy/nullsec/main/install.sh | sh
#
# Linux and macOS only — on Windows, grab the .exe directly from
# https://github.com/nullsec-proxy/nullsec/releases/latest instead.
set -eu

REPO="nullsec-proxy/nullsec"
INSTALL_DIR="${NULLSEC_INSTALL_DIR:-/usr/local/bin}"

os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *)
      echo "error: unsupported OS $(uname -s) — see https://github.com/${REPO}/releases/latest" >&2
      exit 1
      ;;
  esac
}

arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo amd64 ;;
    arm64 | aarch64) echo arm64 ;;
    *)
      echo "error: unsupported architecture $(uname -m) — see https://github.com/${REPO}/releases/latest" >&2
      exit 1
      ;;
  esac
}

goos=$(os)
goarch=$(arch)
bin="nullsec-server-${goos}-${goarch}"
base_url="https://github.com/${REPO}/releases/latest/download"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading ${bin}..."
curl -fsSL "${base_url}/${bin}" -o "${tmp}/${bin}"

echo "Verifying checksum..."
if curl -fsSL "${base_url}/checksums.txt" -o "${tmp}/checksums.txt" 2>/dev/null; then
  expected=$(grep " ${bin}\$" "${tmp}/checksums.txt" | awk '{print $1}')
  actual=$(sha256sum "${tmp}/${bin}" | awk '{print $1}')
  if [ -z "${expected}" ]; then
    echo "warning: ${bin} not listed in checksums.txt — skipping verification" >&2
  elif [ "${expected}" != "${actual}" ]; then
    echo "error: checksum mismatch for ${bin} (expected ${expected}, got ${actual}) — aborting" >&2
    exit 1
  fi
else
  echo "warning: could not fetch checksums.txt — skipping verification" >&2
fi

chmod +x "${tmp}/${bin}"

if [ -w "${INSTALL_DIR}" ]; then
  mv "${tmp}/${bin}" "${INSTALL_DIR}/nullsec-server"
else
  echo "Need sudo to write to ${INSTALL_DIR}..."
  sudo mv "${tmp}/${bin}" "${INSTALL_DIR}/nullsec-server"
fi

echo "Installed to ${INSTALL_DIR}/nullsec-server"
"${INSTALL_DIR}/nullsec-server" version
