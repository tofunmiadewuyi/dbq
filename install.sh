#!/usr/bin/env bash
# Install the dbq CLI to /usr/local/bin (falls back to ~/.local/bin).
# Usage: curl -fsSL https://dbq.tofunmiadewuyi.com/install.sh | bash
#   pin a version: DBQ_VERSION=v1.2.3 ... | bash
#   override artifacts: DBQ_RELEASE_BASE=https://dbq.tofunmiadewuyi.com/releases/dbq
set -euo pipefail

REPO="tofunmiadewuyi/dbq"
DEFAULT_RELEASE_BASE="https://dbq.tofunmiadewuyi.com/releases/dbq"
RELEASE_BASE="${DBQ_RELEASE_BASE:-$DEFAULT_RELEASE_BASE}"
RELEASE_BASE="${RELEASE_BASE%/}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux | darwin) ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  arm64 | aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Resolve version: explicit pin, else newest dbq release.
VERSION="${DBQ_VERSION:-}"
if [ -z "$VERSION" ]; then
  if [[ "$RELEASE_BASE" == https://github.com/*/releases/download ]]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d '"' -f4)
  else
    VERSION=$(curl -fsSL "$RELEASE_BASE/latest.txt" | tr -d '[:space:]')
  fi
fi
if [ -z "$VERSION" ]; then
  echo "could not determine latest dbq release; set DBQ_VERSION=vX.Y.Z" >&2
  exit 1
fi

FILENAME="dbq_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="$RELEASE_BASE/${VERSION}/${FILENAME}"

echo "Installing dbq $VERSION for $OS/$ARCH..."

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fSL "$URL" -o "$TMP_DIR/dbq.tar.gz"
tar -xzf "$TMP_DIR/dbq.tar.gz" -C "$TMP_DIR"
chmod +x "$TMP_DIR/dbq"

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
  echo "Installing to $INSTALL_DIR (no sudo access)"

  SHELL_RC=""
  if [ -f "$HOME/.zshrc" ]; then
    SHELL_RC="$HOME/.zshrc"
  elif [ -f "$HOME/.bashrc" ]; then
    SHELL_RC="$HOME/.bashrc"
  elif [ -f "$HOME/.profile" ]; then
    SHELL_RC="$HOME/.profile"
  fi

  if [ -n "$SHELL_RC" ] && ! grep -q 'HOME/.local/bin' "$SHELL_RC" 2>/dev/null; then
    echo '' >> "$SHELL_RC"
    echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_RC"
    echo "Added ~/.local/bin to PATH in $SHELL_RC"
  fi
fi

mv "$TMP_DIR/dbq" "$INSTALL_DIR/dbq"
echo "Installed to $INSTALL_DIR/dbq"

if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
  echo "Reload your shell or run: export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo "Run: dbq"
