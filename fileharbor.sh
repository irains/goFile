#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  printf 'Please run this installer as root (for example: sudo ./fileharbor.sh).\n' >&2
  exit 1
fi

repo='irains/goFile'
version='latest'

usage() {
  cat <<'EOF'
Usage: fileharbor.sh [--version <tag>]

Installs a verified FileHarbor release binary into /usr/local/bin/fileharbor.
Set --version to a release tag such as v1.4.0; the default is latest.
EOF
}

while (($#)); do
  case "$1" in
    --version)
      (($# >= 2)) || { usage >&2; exit 2; }
      version=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

case "$(uname -s)" in
  Linux) goos='linux' ;;
  Darwin) goos='darwin' ;;
  *) printf 'Unsupported operating system: %s\n' "$(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) goarch='amd64' ;;
  aarch64|arm64) goarch='arm64' ;;
  i386|i686) goarch='386' ;;
  *) printf 'Unsupported architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac

checksum_command=''
case "$goos" in
  linux)
    checksum_command='sha256sum'
    ;;
  darwin)
    checksum_command='shasum'
    ;;
esac

for command in curl "$checksum_command" tar mktemp install; do
  command -v "$command" >/dev/null 2>&1 || {
    printf 'Required command is unavailable: %s\n' "$command" >&2
    exit 1
  }
done

asset="fileharbor-${goos}-${goarch}.tar.gz"
base_url="https://github.com/${repo}/releases/${version}/download"
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT

curl --fail --location --retry 3 --proto '=https' --tlsv1.2 \
  --output "$temporary/$asset" "$base_url/$asset"
curl --fail --location --retry 3 --proto '=https' --tlsv1.2 \
  --output "$temporary/$asset.sha256" "$base_url/$asset.sha256"

(
  cd "$temporary"
  if [ "$goos" = 'darwin' ]; then
    expected=$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1; exit }' "$asset.sha256")
    actual=$(shasum -a 256 "$asset" | awk '{print $1}')
    [ -n "$expected" ] && [ "$actual" = "$expected" ] || {
      printf 'SHA-256 checksum verification failed.\n' >&2
      exit 1
    }
  else
    sha256sum --check "$asset.sha256"
  fi
)
tar -xzf "$temporary/$asset" -C "$temporary" fileharbor
install -o root -g root -m 0755 "$temporary/fileharbor" /usr/local/bin/fileharbor
printf 'FileHarbor installed at /usr/local/bin/fileharbor\n'
printf 'Next: run "fileharbor hash-password", then configure FILEHARBOR_* credentials.\n'
