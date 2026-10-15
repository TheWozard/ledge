#!/usr/bin/env bash
# Install the latest ledge release: download it for this Mac, check it against the
# release's checksums.txt, and put the binary in ~/.local/bin.
#
#   curl -fsSL https://raw.githubusercontent.com/TheWozard/ledge/main/scripts/install.sh | bash
#
# LEDGE_INSTALL_DIR installs somewhere else. Once installed, `ledge update` keeps it up
# to date. LEDGE_GITHUB_API replaces https://api.github.com, for testing.

set -euo pipefail

repo="TheWozard/ledge"
binary="ledge"
install_dir="${LEDGE_INSTALL_DIR:-$HOME/.local/bin}"
api="${LEDGE_GITHUB_API:-https://api.github.com}"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "error: ledge only runs on macOS" >&2
  exit 1
fi
case "$(uname -m)" in
  x86_64) arch="amd64" ;;
  arm64)  arch="arm64" ;;
  *) echo "error: unsupported architecture $(uname -m) (arm64 and amd64 are released)" >&2; exit 1 ;;
esac
asset="${binary}_darwin_${arch}.tar.gz"

echo "Finding the latest release of ${repo}..."
release_json="$(curl -fsSL -H "Accept: application/vnd.github+json" "${api%/}/repos/${repo}/releases/latest")"

tag="$(printf '%s' "$release_json" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
asset_url="$(printf '%s' "$release_json" | grep -o "\"browser_download_url\": *\"[^\"]*/${asset}\"" | sed -E 's/.*"([^"]+)"$/\1/' || true)"
checksums_url="$(printf '%s' "$release_json" | grep -o '"browser_download_url": *"[^"]*/checksums.txt"' | sed -E 's/.*"([^"]+)"$/\1/' || true)"

if [[ -z "$tag" || -z "$asset_url" ]]; then
  echo "error: the latest release of ${repo} has no ${asset}" >&2
  exit 1
fi
if [[ -z "$checksums_url" ]]; then
  echo "error: ${tag} has no checksums.txt to check ${asset} against" >&2
  exit 1
fi

echo "Installing ${binary} ${tag} (darwin/${arch})..."

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

curl -fsSL "$asset_url" -o "${work_dir}/${asset}"
curl -fsSL "$checksums_url" -o "${work_dir}/checksums.txt"
want="$(grep " ${asset}\$" "${work_dir}/checksums.txt" | awk '{print $1}' || true)"
got="$(shasum -a 256 "${work_dir}/${asset}" | awk '{print $1}')"
if [[ -z "$want" || "$want" != "$got" ]]; then
  echo "error: checksum mismatch for ${asset} (checksums.txt has ${want:-none}, the download is ${got})" >&2
  exit 1
fi

tar -xzf "${work_dir}/${asset}" -C "$work_dir" "$binary"
mkdir -p "$install_dir"
mv "${work_dir}/${binary}" "${install_dir}/${binary}"
chmod +x "${install_dir}/${binary}"

echo "Installed ${binary} ${tag} to ${install_dir}/${binary}"
case ":$PATH:" in
  *":${install_dir}:"*) ;;
  *) echo "note: ${install_dir} isn't on your \$PATH; add \`export PATH=\"\$PATH:${install_dir}\"\` to your shell profile" ;;
esac
