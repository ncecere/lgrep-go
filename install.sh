#!/bin/bash
#
# lgrep installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/nickcecere/lgrep/main/install.sh | bash
#

set -e

REPO="nickcecere/lgrep"
BINARY_NAME="lgrep"
INSTALL_DIR="/usr/local/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Get the latest release version from GitHub
get_latest_version() {
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        error "Failed to get latest version from GitHub"
    fi
    info "Latest version: $VERSION"
}

# Download and install the binary
install_binary() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${PLATFORM}.tar.gz"
    TMP_DIR=$(mktemp -d)
    
    info "Downloading from: $DOWNLOAD_URL"
    
    if ! curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${BINARY_NAME}.tar.gz"; then
        error "Failed to download binary. Check if release exists for your platform."
    fi

    info "Extracting..."
    tar -xzf "${TMP_DIR}/${BINARY_NAME}.tar.gz" -C "$TMP_DIR"

    info "Installing to ${INSTALL_DIR}/${BINARY_NAME}"
    
    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMP_DIR}/${BINARY_NAME}-${PLATFORM}" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        warn "Requires sudo to install to ${INSTALL_DIR}"
        sudo mv "${TMP_DIR}/${BINARY_NAME}-${PLATFORM}" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # Cleanup
    rm -rf "$TMP_DIR"

    info "Installation complete!"
}

# Verify the installation
verify_installation() {
    if command -v "$BINARY_NAME" &> /dev/null; then
        info "Verifying installation..."
        "$BINARY_NAME" version
        echo ""
        info "Successfully installed $BINARY_NAME!"
        echo ""
        echo "Get started:"
        echo "  lgrep index           # Index current directory"
        echo "  lgrep \"your query\"    # Search for code"
        echo "  lgrep --help          # Show all commands"
    else
        warn "Installation completed but '$BINARY_NAME' not found in PATH"
        warn "You may need to add ${INSTALL_DIR} to your PATH"
    fi
}

main() {
    echo ""
    echo "  _                       "
    echo " | |  __ _ _ __ ___ _ __  "
    echo " | | / _\` | '__/ _ \\ '_ \\ "
    echo " | || (_| | | |  __/ |_) |"
    echo " |_| \\__, |_|  \\___| .__/ "
    echo "     |___/         |_|    "
    echo ""
    echo "Local Semantic Code Search"
    echo ""

    detect_platform
    get_latest_version
    install_binary
    verify_installation
}

main
