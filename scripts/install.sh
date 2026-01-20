#!/usr/bin/env bash
#
# Tasker installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/dgordon/tasker/main/scripts/install.sh | bash
#
# This script must be EXECUTED, never SOURCED
# WRONG: source install.sh (will exit your shell on errors)
# CORRECT: bash install.sh
# CORRECT: curl -fsSL ... | bash
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}==>${NC} $1"
}

log_success() {
    echo -e "${GREEN}==>${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}==>${NC} $1"
}

log_error() {
    echo -e "${RED}Error:${NC} $1" >&2
}

release_has_asset() {
    local release_json=$1
    local asset_name=$2

    if echo "$release_json" | grep -Fq "\"name\": \"$asset_name\""; then
        return 0
    fi

    return 1
}

resign_for_macos() {
    local binary_path=$1

    if [[ "$(uname -s)" != "Darwin" ]]; then
        return 0
    fi

    if ! command -v codesign &> /dev/null; then
        log_warning "codesign not found, skipping re-signing"
        return 0
    fi

    log_info "Re-signing binary for macOS..."
    codesign --remove-signature "$binary_path" 2>/dev/null || true
    if codesign --force --sign - "$binary_path"; then
        log_success "Binary re-signed for this machine"
    else
        log_warning "Failed to re-sign binary (non-fatal)"
    fi
}

detect_platform() {
    local os arch

    case "$(uname -s)" in
        Darwin)
            os="darwin"
            ;;
        Linux)
            os="linux"
            ;;
        FreeBSD)
            os="freebsd"
            ;;
        *)
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        armv7*|armv6*|armhf|arm)
            arch="arm"
            ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac

    echo "${os}_${arch}"
}

install_from_release() {
    log_info "Installing tasker from GitHub releases..."

    local platform=$1
    local tmp_dir
    tmp_dir=$(mktemp -d)

    log_info "Fetching latest release..."
    local latest_url="https://api.github.com/repos/dgordon/tasker/releases/latest"
    local version
    local release_json

    if command -v curl &> /dev/null; then
        release_json=$(curl -fsSL "$latest_url")
    elif command -v wget &> /dev/null; then
        release_json=$(wget -qO- "$latest_url")
    else
        log_error "Neither curl nor wget found. Please install one of them."
        return 1
    fi

    version=$(echo "$release_json" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')

    if [ -z "$version" ]; then
        log_error "Failed to fetch latest version"
        return 1
    fi

    log_info "Latest version: $version"

    local archive_name="tasker_${version#v}_${platform}.tar.gz"
    local download_url="https://github.com/dgordon/tasker/releases/download/${version}/${archive_name}"

    if ! release_has_asset "$release_json" "$archive_name"; then
        log_warning "No prebuilt archive available for platform ${platform}. Falling back to source installation methods."
        rm -rf "$tmp_dir"
        return 1
    fi

    log_info "Downloading $archive_name..."

    cd "$tmp_dir"
    if command -v curl &> /dev/null; then
        if ! curl -fsSL -o "$archive_name" "$download_url"; then
            log_error "Download failed"
            cd - > /dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 1
        fi
    elif command -v wget &> /dev/null; then
        if ! wget -q -O "$archive_name" "$download_url"; then
            log_error "Download failed"
            cd - > /dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 1
        fi
    fi

    log_info "Extracting archive..."
    if ! tar -xzf "$archive_name"; then
        log_error "Failed to extract archive"
        rm -rf "$tmp_dir"
        return 1
    fi

    local install_dir
    if [[ -w /usr/local/bin ]]; then
        install_dir="/usr/local/bin"
    else
        install_dir="$HOME/.local/bin"
        mkdir -p "$install_dir"
    fi

    log_info "Installing to $install_dir..."
    if [[ -w "$install_dir" ]]; then
        mv tasker "$install_dir/"
    else
        sudo mv tasker "$install_dir/"
    fi

    resign_for_macos "$install_dir/tasker"

    log_success "tasker installed to $install_dir/tasker"

    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        log_warning "$install_dir is not in your PATH"
        echo ""
        echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo "  export PATH=\"\$PATH:$install_dir\""
        echo ""
    fi

    cd - > /dev/null || cd "$HOME"
    rm -rf "$tmp_dir"
    return 0
}

check_go() {
    if command -v go &> /dev/null; then
        local go_version=$(go version | awk '{print $3}' | sed 's/go//')
        log_info "Go detected: $(go version)"

        local major=$(echo "$go_version" | cut -d. -f1)
        local minor=$(echo "$go_version" | cut -d. -f2)

        if [ "$major" -eq 1 ] && [ "$minor" -lt 24 ]; then
            log_error "Go 1.24 or later is required (found: $go_version)"
            echo ""
            echo "Please upgrade Go:"
            echo "  - Download from https://go.dev/dl/"
            echo "  - Or use your package manager to update"
            echo ""
            return 1
        fi

        return 0
    else
        return 1
    fi
}

install_with_go() {
    log_info "Installing tasker using 'go install'..."

    if go install github.com/dgordon/tasker/go/cmd/tasker@latest; then
        log_success "tasker installed successfully via go install"

        local gobin
        gobin=$(go env GOBIN 2>/dev/null || true)
        if [ -n "$gobin" ]; then
            bin_dir="$gobin"
        else
            bin_dir="$(go env GOPATH)/bin"
        fi
        LAST_INSTALL_PATH="$bin_dir/tasker"

        resign_for_macos "$bin_dir/tasker"

        if [[ ":$PATH:" != *":$bin_dir:"* ]]; then
            log_warning "$bin_dir is not in your PATH"
            echo ""
            echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
            echo "  export PATH=\"\$PATH:$bin_dir\""
            echo ""
        fi

        return 0
    else
        log_error "go install failed"
        return 1
    fi
}

build_from_source() {
    log_info "Building tasker from source..."

    local tmp_dir
    tmp_dir=$(mktemp -d)

    cd "$tmp_dir"
    log_info "Cloning repository..."

    if git clone --depth 1 https://github.com/dgordon/tasker.git; then
        cd tasker/go
        log_info "Building binary..."

        local version=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
        local commit=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
        local build_date=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
        local ldflags="-X main.version=${version} -X main.commit=${commit} -X main.date=${build_date}"

        if go build -ldflags "$ldflags" -o tasker ./cmd/tasker; then
            local install_dir
            if [[ -w /usr/local/bin ]]; then
                install_dir="/usr/local/bin"
            else
                install_dir="$HOME/.local/bin"
                mkdir -p "$install_dir"
            fi

            log_info "Installing to $install_dir..."
            if [[ -w "$install_dir" ]]; then
                mv tasker "$install_dir/"
            else
                sudo mv tasker "$install_dir/"
            fi

            resign_for_macos "$install_dir/tasker"

            log_success "tasker installed to $install_dir/tasker"

            LAST_INSTALL_PATH="$install_dir/tasker"

            if [[ ":$PATH:" != *":$install_dir:"* ]]; then
                log_warning "$install_dir is not in your PATH"
                echo ""
                echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
                echo "  export PATH=\"\$PATH:$install_dir\""
                echo ""
            fi

            cd - > /dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 0
        else
            log_error "Build failed"
            cd - > /dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 1
        fi
    else
        log_error "Failed to clone repository"
        rm -rf "$tmp_dir"
        return 1
    fi
}

get_tasker_paths_in_path() {
    local IFS=':'
    local -a entries
    read -ra entries <<< "$PATH"
    local -a found
    local p
    for p in "${entries[@]}"; do
        [ -z "$p" ] && continue
        if [ -x "$p/tasker" ]; then
            if command -v readlink >/dev/null 2>&1; then
                resolved=$(readlink -f "$p/tasker" 2>/dev/null || printf '%s' "$p/tasker")
            else
                resolved="$p/tasker"
            fi
            skip=0
            for existing in "${found[@]:-}"; do
                if [ "$existing" = "$resolved" ]; then skip=1; break; fi
            done
            if [ $skip -eq 0 ]; then
                found+=("$resolved")
            fi
        fi
    done
    for item in "${found[@]:-}"; do
        printf '%s\n' "$item"
    done
}

warn_if_multiple_tasker() {
    tasker_paths=()
    while IFS= read -r line; do
        tasker_paths+=("$line")
    done < <(get_tasker_paths_in_path)
    if [ "${#tasker_paths[@]}" -le 1 ]; then
        return 0
    fi

    log_warning "Multiple 'tasker' executables found on your PATH. An older copy may be executed instead of the one we installed."
    echo "Found the following 'tasker' executables (entries earlier in PATH take precedence):"
    local i=1
    for p in "${tasker_paths[@]}"; do
        local ver
        if [ -x "$p" ]; then
            ver=$("$p" --version 2>/dev/null || true)
        fi
        if [ -z "$ver" ]; then ver="<unknown version>"; fi
        echo "  $i. $p  -> $ver"
        i=$((i+1))
    done

    if [ -n "$LAST_INSTALL_PATH" ]; then
        echo ""
        echo "We installed to: $LAST_INSTALL_PATH"
        first="${tasker_paths[0]}"
        if [ "$first" != "$LAST_INSTALL_PATH" ]; then
            log_warning "The 'tasker' executable that appears first in your PATH is different from the one we installed. To make the newly installed 'tasker' the one you get when running 'tasker', either:"
            echo "  - Remove or rename the older $first from your PATH, or"
            echo "  - Reorder your PATH so that $(dirname "$LAST_INSTALL_PATH") appears before $(dirname "$first")"
            echo "After updating PATH, restart your shell and run 'tasker --version' to confirm."
        else
            echo "The installed 'tasker' is first in your PATH.";
        fi
    else
        log_warning "We couldn't determine where we installed 'tasker' during this run.";
    fi
}

verify_installation() {
    warn_if_multiple_tasker || true

    if command -v tasker &> /dev/null; then
        log_success "tasker is installed and ready!"
        echo ""
        tasker --version 2>/dev/null || echo "tasker (development build)"
        echo ""
        echo "Get started:"
        echo "  tasker --help"
        echo ""
        return 0
    else
        log_error "tasker was installed but is not in PATH"
        return 1
    fi
}

main() {
    echo ""
    echo "Tasker Installer"
    echo ""

    log_info "Detecting platform..."
    local platform
    platform=$(detect_platform)
    log_info "Platform: $platform"

    if install_from_release "$platform"; then
        verify_installation
        exit 0
    fi

    log_warning "Failed to install from releases, trying alternative methods..."

    if check_go; then
        if install_with_go; then
            verify_installation
            exit 0
        fi
    fi

    log_warning "Falling back to building from source..."

    if ! check_go; then
        log_warning "Go is not installed"
        echo ""
        echo "tasker requires Go 1.24 or later to build from source. You can:"
        echo "  1. Install Go from https://go.dev/dl/"
        echo "  2. Use your package manager:"
        echo "     - macOS: brew install go"
        echo "     - Ubuntu/Debian: sudo apt install golang"
        echo "     - Other Linux: Check your distro's package manager"
        echo ""
        echo "After installing Go, run this script again."
        exit 1
    fi

    if build_from_source; then
        verify_installation
        exit 0
    fi

    log_error "Installation failed"
    echo ""
    echo "Manual installation:"
    echo "  1. Download from https://github.com/dgordon/tasker/releases/latest"
    echo "  2. Extract and move 'tasker' to your PATH"
    echo ""
    echo "Or install from source:"
    echo "  1. Install Go from https://go.dev/dl/"
    echo "  2. Run: go install github.com/dgordon/tasker/go/cmd/tasker@latest"
    echo ""
    exit 1
}

main "$@"
