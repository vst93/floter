#!/usr/bin/env bash
#
# floter installer for macOS and Linux.
#
# Examples:
#   curl -fsSL https://raw.githubusercontent.com/vst93/floter/main/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/vst93/floter/main/scripts/install.sh | bash -s -- --pre-release
#   curl -fsSL https://raw.githubusercontent.com/vst93/floter/main/scripts/install.sh | bash -s -- --version 0.3.0

set -euo pipefail

REPO="vst93/floter"
OPT_VERSION=""
OPT_PRE_RELEASE=false
OPT_YES=false
TEMP_DIR=""

if [[ -t 2 ]]; then
  BLUE='\033[1;34m'
  YELLOW='\033[1;33m'
  RED='\033[1;31m'
  RESET='\033[0m'
else
  BLUE=''
  YELLOW=''
  RED=''
  RESET=''
fi

info()  { printf "%b==>%b %s\n" "$BLUE" "$RESET" "$*" >&2; }
warn()  { printf "%b!!%b %s\n" "$YELLOW" "$RESET" "$*" >&2; }
error() { printf "%b!!%b %s\n" "$RED" "$RESET" "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || error "Missing required command: '$1'. Please install it and try again."
}

cleanup() {
  if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
    rm -rf "$TEMP_DIR"
  fi
}
trap cleanup EXIT

usage() {
  cat <<'EOF'
floter installer

Usage:
  install.sh [options]

Options:
  -v, --version <tag>  Install a specific version (e.g. 0.3.0)
  -p, --pre-release    Install the latest release (including previews)
  -y, --yes            Skip confirmation prompts (for automation)
  -h, --help           Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -v|--version)
      [[ $# -ge 2 && -n "$2" && "$2" != -* ]] || error "$1 requires a version argument."
      OPT_VERSION="$2"
      shift 2
      ;;
    -p|--pre-release)
      OPT_PRE_RELEASE=true
      shift
      ;;
    -y|--yes)
      OPT_YES=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *) error "Unknown argument: $1 (use --help for usage)" ;;
  esac
done

if [[ -n "$OPT_VERSION" && "$OPT_PRE_RELEASE" == "true" ]]; then
  error "--version and --pre-release cannot be used together."
fi

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "macos" ;;
    Linux) echo "linux" ;;
    *) error "Unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "x86_64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) error "Unsupported architecture: $(uname -m)" ;;
  esac
}

resolve_tag() {
  if [[ -n "$OPT_VERSION" ]]; then
    info "Specified version: ${OPT_VERSION}"
    printf '%s\n' "$OPT_VERSION"
    return
  fi

  local api_url response tag
  if [[ "$OPT_PRE_RELEASE" == "true" ]]; then
    api_url="https://api.github.com/repos/${REPO}/releases?per_page=20"
  else
    api_url="https://api.github.com/repos/${REPO}/releases/latest"
  fi

  response="$(curl -fsSL --retry 3 --connect-timeout 15 "$api_url")" \
    || error "Cannot reach GitHub Releases. Check your network or use --version to specify a version."
  tag="$(sed -nE 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' <<< "$response" | sed -n '1p')"
  [[ -n "$tag" ]] || error "Could not parse version from GitHub Releases."

  if [[ "$OPT_PRE_RELEASE" == "true" ]]; then
    info "Latest release (including previews): ${tag}"
  else
    info "Latest stable: ${tag}"
  fi
  printf '%s\n' "$tag"
}

make_temp_dir() {
  TEMP_DIR="$(mktemp -d)"
}

download_asset() {
  local url="$1" destination="$2"
  info "Downloading $(basename "$destination") ..."
  if ! curl -fL --retry 3 --connect-timeout 15 --show-error --progress-bar \
    -o "$destination" "$url"; then
    error "Download failed: ${url}\nMake sure this version has an installer for your platform."
  fi
}

confirm_action() {
  local prompt="$1" reply
  [[ "$OPT_YES" == "true" ]] && return

  printf "%b==>%b %s [Y/n] " "$BLUE" "$RESET" "$prompt" >/dev/tty 2>/dev/null \
    || error "Non-interactive environment; use --yes for automated installs."
  if ! IFS= read -r reply </dev/tty; then
    error "Cannot read confirmation; use --yes for automated installs."
  fi
  case "$reply" in
    ''|y|Y|yes|YES) ;;
    *) error "Installation cancelled." ;;
  esac
}

as_root() {
  if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    "$@"
  else
    need sudo
    sudo "$@"
  fi
}

install_macos() {
  local arch="$1" tag="$2" asset_prefix dmg_url dmg mountpoint

  case "$arch" in
    arm64) asset_prefix="floter-${tag}-macos-apple-silicon" ;;
    x86_64) asset_prefix="floter-${tag}-macos-intel" ;;
  esac

  info "Detected macOS (${arch})"
  if [[ -z "$OPT_VERSION" && "$OPT_PRE_RELEASE" == "false" ]] \
    && command -v brew >/dev/null 2>&1; then
    info "Installing via Homebrew. Run 'brew upgrade --cask floter' to update later."
    brew install --cask floter 2>/dev/null || {
      warn "Homebrew cask not available, falling back to .dmg download."
    }
    if command -v floter >/dev/null 2>&1; then
      info "Done."
      return
    fi
  fi

  need hdiutil
  make_temp_dir
  dmg_url="https://github.com/${REPO}/releases/download/${tag}/${asset_prefix}.dmg"
  dmg="${TEMP_DIR}/${asset_prefix}.dmg"
  download_asset "$dmg_url" "$dmg"

  mountpoint="$(hdiutil attach "$dmg" -nobrowse -quiet | sed -n 's#^.*\(/Volumes/.*\)$#\1#p' | tail -1)"
  [[ -n "$mountpoint" && -d "${mountpoint}/floter.app" ]] || error "Could not mount disk image."
  confirm_action "Install floter to /Applications?"
  as_root ditto "${mountpoint}/floter.app" /Applications/floter.app
  hdiutil detach "$mountpoint" -quiet
  as_root xattr -dr com.apple.quarantine /Applications/floter.app 2>/dev/null || true
  info "Done. Open floter from Applications."
}

install_linux_deb() {
  local arch="$1" tag="$2" package_name package_url package
  case "$arch" in
    arm64) package_name="floter-${tag}-linux-arm64.deb" ;;
    x86_64) package_name="floter-${tag}-linux-x86_64.deb" ;;
  esac
  package_url="https://github.com/${REPO}/releases/download/${tag}/${package_name}"

  info "Detected Debian / Ubuntu"
  need apt-get
  make_temp_dir
  package="${TEMP_DIR}/${package_name}"
  download_asset "$package_url" "$package"
  confirm_action "Install ${package_name} and dependencies via apt?"
  as_root apt-get install -y "$package"
  info "Done. Re-run this script to upgrade."
}

install_linux_arch() {
  local arch="$1" tag="$2" deb_name deb_url deb
  case "$arch" in
    arm64) deb_name="floter-${tag}-linux-arm64.deb" ;;
    x86_64) deb_name="floter-${tag}-linux-x86_64.deb" ;;
  esac
  deb_url="https://github.com/${REPO}/releases/download/${tag}/${deb_name}"

  info "Detected Arch Linux / pacman"

  make_temp_dir
  deb="${TEMP_DIR}/${deb_name}"
  download_asset "$deb_url" "$deb"

  confirm_action "Build and install floter-bin from the downloaded .deb?"
  (
    cd "$TEMP_DIR"
    rm -rf pkgbuild && mkdir pkgbuild && cd pkgbuild
    cat > PKGBUILD <<PKGEOF
# Generated by floter install script — repackages the official release .deb
pkgname=floter-bin
_pkgname=floter
pkgver=${tag}
pkgrel=1
pkgdesc="A floating terminal and application launcher"
arch=('${arch}')
url="https://github.com/vst93/floter"
license=('GPL-3.0-or-later')
depends=('webkit2gtk-4.1' 'gtk3' 'libappindicator-gtk3' 'librsvg' 'xdg-utils')
provides=("\$_pkgname")
conflicts=("\$_pkgname")
options=('!strip' '!emptydirs')
source=("\$_pkgname-\${pkgver}-${arch}.deb::${deb_url}")
sha256sums=('SKIP')

prepare() {
  bsdtar -xf "\$srcdir"/*.deb -C "\$srcdir"
}

package() {
  bsdtar -xf "\$srcdir"/data.tar.* -C "\$pkgdir"
}
PKGEOF
    if [[ "$OPT_YES" == "true" ]]; then
      makepkg -si --noconfirm
    else
      makepkg -si
    fi
  )
  info "Done. Re-run this script to upgrade. Uninstall: sudo pacman -R floter-bin"
}

install_linux() {
  local arch="$1" tag="$2"
  info "Detected Linux (${arch})"

  if command -v pacman >/dev/null 2>&1; then
    install_linux_arch "$arch" "$tag"
  elif command -v apt-get >/dev/null 2>&1 && command -v dpkg >/dev/null 2>&1; then
    install_linux_deb "$arch" "$tag"
  else
    error "This Linux distribution is not yet supported by the install script.\nSupported: pacman (Arch/CachyOS/Manjaro), apt (Debian/Ubuntu).\nYou can download an installer manually from https://github.com/${REPO}/releases"
  fi
}

main() {
  local os arch tag
  need curl
  os="$(detect_os)"
  arch="$(detect_arch)"
  tag="$(resolve_tag)"
  info "floter installer"

  case "$os" in
    macos) install_macos "$arch" "$tag" ;;
    linux) install_linux "$arch" "$tag" ;;
  esac
}

main
