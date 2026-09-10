#!/usr/bin/env bash
# Install Daily Work from GitHub Releases (no Go required).
#
# One-liner (after the repo is public):
#   curl -fsSL https://raw.githubusercontent.com/<YOUR_USER>/daily-work/main/install.sh | bash
#
# From a clone (dev / fallback build):
#   ./install.sh --from-source
set -euo pipefail

# --- set automatically when you publish; override with env if needed ---
GITHUB_OWNER="${DAILY_WORK_GITHUB_OWNER:-7AkhilV}"
GITHUB_REPO="${DAILY_WORK_GITHUB_REPO:-daily-work}"
# -----------------------------------------------------------------------

APP="daily-work"
MODEL="llama3.2:3b"
BIN_DIR="${HOME}/.local/bin"
INSTALL_PATH="${BIN_DIR}/${APP}"
FROM_SOURCE=0

blue()  { printf '\033[1;34m%s\033[0m\n' "$*"; }
green() { printf '\033[1;32m%s\033[0m\n' "$*"; }
yellow(){ printf '\033[1;33m%s\033[0m\n' "$*"; }
red()   { printf '\033[1;31m%s\033[0m\n' "$*"; }

for arg in "$@"; do
  case "$arg" in
    --from-source) FROM_SOURCE=1 ;;
    --help|-h)
      echo "Usage: ./install.sh [--from-source]"
      exit 0
      ;;
  esac
done

ensure_path() {
  mkdir -p "${BIN_DIR}"
  export PATH="${BIN_DIR}:${PATH}"
  for rc in "${HOME}/.zshrc" "${HOME}/.bashrc"; do
    if [[ -f "$rc" ]] || [[ ! -e "$rc" && "$rc" == "${HOME}/.zshrc" ]]; then
      touch "$rc" 2>/dev/null || true
      if [[ -f "$rc" ]] && ! grep -q 'export PATH="$HOME/.local/bin:$PATH"' "$rc" 2>/dev/null; then
        printf '\n# daily-work\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$rc"
        green "Updated $rc (PATH)"
      fi
    fi
  done
}

detect_asset() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$os" in
    darwin) os="darwin" ;;
    linux) os="linux" ;;
    *) red "Unsupported OS: $os"; exit 1 ;;
  esac
  case "$arch" in
    arm64|aarch64) arch="arm64" ;;
    x86_64|amd64) arch="amd64" ;;
    *) red "Unsupported arch: $arch"; exit 1 ;;
  esac
  echo "daily-work-${os}-${arch}"
}

install_from_release() {
  if [[ "$GITHUB_OWNER" == "YOUR_GITHUB_USERNAME" ]]; then
    red "install.sh still has YOUR_GITHUB_USERNAME — set your personal GitHub user first."
    echo "Or run: DAILY_WORK_GITHUB_OWNER=youruser ./install.sh"
    echo "Dev fallback: ./install.sh --from-source"
    exit 1
  fi

  need_cmd() { command -v "$1" >/dev/null 2>&1 || { red "Need $1"; exit 1; }; }
  need_cmd curl

  local asset version url tmp
  asset="$(detect_asset)"
  blue "→ Fetching latest release from ${GITHUB_OWNER}/${GITHUB_REPO}..."
  version="$(curl -fsSL "https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
  if [[ -z "$version" ]]; then
    red "No GitHub release found. Create a tag like v0.1.0 after pushing the repo."
    echo "Fallback: ./install.sh --from-source"
    exit 1
  fi
  url="https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases/download/${version}/${asset}"
  tmp="$(mktemp)"
  yellow "Downloading ${asset} (${version})..."
  curl -fsSL -o "$tmp" "$url"
  chmod +x "$tmp"
  mv "$tmp" "${INSTALL_PATH}"
  green "✓ Installed ${INSTALL_PATH} (${version})"
}

install_from_source() {
  local root
  root="$(cd "$(dirname "$0")" && pwd)"
  if ! command -v go >/dev/null 2>&1; then
    red "Go is required for --from-source. Install from https://go.dev/dl/"
    exit 1
  fi
  blue "→ Building from source..."
  mkdir -p "${root}/bin"
  (cd "${root}" && go build -o "bin/${APP}" ./cmd/daily-work)
  cp "${root}/bin/${APP}" "${INSTALL_PATH}"
  chmod +x "${INSTALL_PATH}"
  green "✓ Installed ${INSTALL_PATH} (from source)"
}

setup_ollama() {
  blue "→ Local AI (Ollama + ${MODEL}, ~2GB)..."
  if ! command -v ollama >/dev/null 2>&1; then
    if command -v brew >/dev/null 2>&1; then
      yellow "Installing Ollama via Homebrew..."
      brew install ollama
    else
      red "Install Ollama from https://ollama.com then re-run install."
      exit 1
    fi
  fi
  if ! curl -sf "http://localhost:11434/api/tags" >/dev/null 2>&1; then
    yellow "Starting Ollama..."
    [[ "$(uname -s)" == "Darwin" ]] && open -a Ollama >/dev/null 2>&1 || true
    if ! curl -sf "http://localhost:11434/api/tags" >/dev/null 2>&1; then
      nohup ollama serve >/dev/null 2>&1 &
    fi
    for _ in $(seq 1 30); do
      curl -sf "http://localhost:11434/api/tags" >/dev/null 2>&1 && break
      sleep 1
    done
  fi
  if ! curl -sf "http://localhost:11434/api/tags" >/dev/null 2>&1; then
    red "Ollama is not reachable. Open the Ollama app, then: ollama pull ${MODEL}"
    exit 1
  fi
  green "✓ Ollama running"
  if ollama list 2>/dev/null | awk '{print $1}' | grep -qx "${MODEL}"; then
    green "✓ Model present: ${MODEL}"
  else
    yellow "Downloading ${MODEL} (~2GB, one-time)..."
    ollama pull "${MODEL}"
    green "✓ Model ready"
  fi
}

print_next() {
  echo
  green "Daily Work installed."
  echo
  echo "Once:  ${APP} auth"
  echo "Daily: ${APP}"
  echo
  echo "Update later: re-run this install script (fetches newest release)."
}

main() {
  echo
  blue "Daily Work — install"
  blue "────────────────────────────────────────"
  ensure_path
  if [[ "$FROM_SOURCE" -eq 1 ]]; then
    install_from_source
  else
    install_from_release
  fi
  setup_ollama
  "${INSTALL_PATH}" setup >/dev/null 2>&1 || true
  print_next
}

main "$@"
