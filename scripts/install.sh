#!/usr/bin/env bash
# Install the latest rdap release for macOS or Linux.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/robkerry/rdap/main/scripts/install.sh | bash
#   RDAP_INSTALL_DIR="$HOME/bin" ./scripts/install.sh

set -euo pipefail

REPO="robkerry/rdap"
BASE_URL="https://github.com/${REPO}/releases/latest/download"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  darwin|linux) ;;
  *)
    echo "Error: unsupported OS '$os'. Download a Windows zip from:" >&2
    echo "  https://github.com/${REPO}/releases/latest" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "Error: unsupported architecture '$arch'." >&2
    exit 1
    ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  echo "Error: curl is required." >&2
  exit 1
fi

install_dir="${RDAP_INSTALL_DIR:-}"
if [ -z "$install_dir" ]; then
  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    install_dir="/usr/local/bin"
  else
    install_dir="${HOME}/.local/bin"
  fi
fi

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT

echo "Downloading checksums..."
curl -fsSL "${BASE_URL}/SHA256SUMS" -o "${tmpdir}/SHA256SUMS"

asset="$(awk -v os="$os" -v arch="$arch" '
  $2 ~ ("^rdap_.+_" os "_" arch "\\.tar\\.gz$") { print $2; exit }
' "${tmpdir}/SHA256SUMS")"

if [ -z "$asset" ]; then
  echo "Error: no release archive found for ${os}/${arch}." >&2
  cat "${tmpdir}/SHA256SUMS" >&2
  exit 1
fi

echo "Downloading ${asset}..."
curl -fsSL "${BASE_URL}/${asset}" -o "${tmpdir}/${asset}"

expected="$(awk -v name="$asset" '$2 == name { print $1; exit }' "${tmpdir}/SHA256SUMS")"
if [ -z "$expected" ]; then
  echo "Error: checksum missing for ${asset}." >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${tmpdir}/${asset}" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "${tmpdir}/${asset}" | awk '{ print $1 }')"
else
  echo "Error: sha256sum or shasum is required." >&2
  exit 1
fi

if [ "$actual" != "$expected" ]; then
  echo "Error: checksum mismatch for ${asset}." >&2
  echo "  expected: ${expected}" >&2
  echo "  actual:   ${actual}" >&2
  exit 1
fi

tar -xzf "${tmpdir}/${asset}" -C "$tmpdir"

if [ ! -f "${tmpdir}/rdap" ]; then
  echo "Error: archive did not contain an rdap binary." >&2
  exit 1
fi

mkdir -p "$install_dir"
install -m 0755 "${tmpdir}/rdap" "${install_dir}/rdap"

echo "Installed ${install_dir}/rdap"
"${install_dir}/rdap" --version

case ":$PATH:" in
  *":${install_dir}:"*) ;;
  *)
    echo
    echo "Note: ${install_dir} is not on your PATH."
    echo "Add it, or re-run with RDAP_INSTALL_DIR pointing at a directory that is."
    ;;
esac
