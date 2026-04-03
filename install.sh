#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-QuinsZouls/code-index}"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BIN_NAME="${BIN_NAME:-codeindex}"

is_windows() {
  case "${OS:-$(uname -s)}" in
    MINGW*|MSYS*|CYGWIN*|Windows_NT) return 0 ;;
    *) return 1 ;;
  esac
}

os_name() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *) echo "unsupported" ;;
  esac
}

arch_name() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) echo "unsupported" ;;
  esac
}

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

github_release_url() {
  local asset="$1"
  local release_path
  if [[ "$VERSION" == "latest" ]]; then
    release_path="latest/download"
  else
    release_path="download/${VERSION}"
  fi
  echo "https://github.com/${REPO}/releases/${release_path}/${asset}"
}

download_unix() {
  need curl
  need tar

  local os arch asset url tmp_dir
  os="$(os_name)"
  arch="$(arch_name)"
  if [[ "$os" == "unsupported" || "$arch" == "unsupported" ]]; then
    echo "unsupported platform: $(uname -s)/$(uname -m)" >&2
    exit 1
  fi

  asset="codeindex-${os}-${arch}.tar.gz"
  url="$(github_release_url "$asset")"
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  mkdir -p "$INSTALL_DIR"
  curl -fsSL "$url" -o "$tmp_dir/$asset"
  tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
  install -m 0755 "$tmp_dir/codeindex-${os}-${arch}" "$INSTALL_DIR/$BIN_NAME"

  echo "Installed $BIN_NAME to $INSTALL_DIR/$BIN_NAME"
}

download_windows() {
  local ps_script tmp_ps1
  need powershell.exe
  tmp_ps1="$(mktemp /tmp/codeindex-install.XXXXXX.ps1)"
  cat > "$tmp_ps1" <<'EOF'
$ErrorActionPreference = 'Stop'
$repo = $env:REPO
$version = $env:VERSION
$binName = $env:BIN_NAME
$installDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\codeindex' }
$arch = if ([Environment]::Is64BitOperatingSystem) { 'amd64' } else { 'amd64' }
$asset = "codeindex-windows-$arch.zip"
$releasePath = if ($version -eq 'latest') { 'latest/download' } else { "download/$version" }
$url = "https://github.com/$repo/releases/$releasePath/$asset"
$tmpDir = New-Item -ItemType Directory -Force -Path ([System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid().ToString())
try {
  New-Item -ItemType Directory -Force -Path $installDir | Out-Null
  $zipPath = Join-Path $tmpDir.FullName $asset
  Invoke-WebRequest -Uri $url -OutFile $zipPath
  Expand-Archive -Path $zipPath -DestinationPath $tmpDir.FullName -Force
  $source = Join-Path $tmpDir.FullName "codeindex-windows-$arch.exe"
  $target = Join-Path $installDir "$binName.exe"
  Copy-Item $source $target -Force
  Write-Host "Installed $binName to $target"
} finally {
  Remove-Item -Recurse -Force $tmpDir.FullName -ErrorAction SilentlyContinue
}
EOF
  REPO="$REPO" VERSION="$VERSION" BIN_NAME="$BIN_NAME" INSTALL_DIR="${INSTALL_DIR:-}" powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$tmp_ps1"
  rm -f "$tmp_ps1"
}

if is_windows; then
  download_windows
else
  download_unix
fi
