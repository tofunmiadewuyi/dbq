#!/usr/bin/env bash
set -e

REPO="tofunmiadewuyi/dbq"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  darwin) OS="darwin" ;;
  linux) OS="linux" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

VERSION=$(curl -s https://api.github.com/repos/$REPO/releases/latest | grep tag_name | cut -d '"' -f4)

FILENAME="dbq_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

echo "Installing dbq $VERSION for $OS/$ARCH..."

TMP_DIR=$(mktemp -d)
curl -L "$URL" -o "$TMP_DIR/dbq.tar.gz"

tar -xzf "$TMP_DIR/dbq.tar.gz" -C "$TMP_DIR"

chmod +x "$TMP_DIR/dbq"

INSTALL_DIR="/usr/local/bin"

if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
  echo "Installing to $INSTALL_DIR (no sudo access)"

  # Add to PATH in shell profile if not already present
  SHELL_RC=""
  if [ -f "$HOME/.zshrc" ]; then
    SHELL_RC="$HOME/.zshrc"
  elif [ -f "$HOME/.bashrc" ]; then
    SHELL_RC="$HOME/.bashrc"
  elif [ -f "$HOME/.profile" ]; then
    SHELL_RC="$HOME/.profile"
  fi

  if [ -n "$SHELL_RC" ]; then
    if ! grep -q 'HOME/.local/bin' "$SHELL_RC" 2>/dev/null; then
      echo '' >> "$SHELL_RC"
      echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_RC"
      echo "Added ~/.local/bin to PATH in $SHELL_RC"
    fi
  fi
fi

mv "$TMP_DIR/dbq" "$INSTALL_DIR/dbq"

echo "Installed to $INSTALL_DIR/dbq"

if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
  echo "Reload your shell or run: export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo "Run: dbq"
