#!/bin/sh
# Blaxel CLI installer
#
#   curl -fsSL https://blaxel.ai/install.sh | sh
#
# Installs bl (and blaxel) to ~/.local/bin, verified against the release
# checksums, adds it to your PATH with shell completions, then runs bl setup
# to set up your coding agents and log you in.
#
# Environment:
#   VERSION=v1.2.3                install that release instead of the latest
#   BINDIR=DIR                    install to DIR instead of ~/.local/bin
#   BL_INSTALL_PATH=false         leave your shell configuration alone
#   BL_INSTALL_COMPLETION=false   skip shell completions
#   BL_INSTALL_SETUP=false        skip bl setup (=true runs it without a terminal)
#   NO_COLOR=1                    plain output
set -e

OWNER=blaxel-ai
REPO=toolkit
BINARY=blaxel
BINARY_SHORT_NAME=bl
# Under sudo, set up the user who ran it: their home, their shell, their files.
SUDO_HOME=""
if [ "$(id -u)" = "0" ] && [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
  case "$SUDO_USER" in
    *[!A-Za-z0-9._-]*) ;;
    *) SUDO_HOME=$(eval "echo ~$SUDO_USER") ;;
  esac
  case "$SUDO_HOME" in /*) HOME=$SUDO_HOME ;; *) SUDO_HOME="" ;; esac
fi
BINDIR=${BINDIR:-$HOME/.local/bin}
RELEASES="https://github.com/$OWNER/$REPO/releases"

# --- Output: styled on a terminal, plain in logs -------------------------

UI_TTY=""
c_accent="" c_ok="" c_fail="" c_muted="" c_reset=""
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
  UI_TTY=1
  esc=$(printf '\033')
  c_reset="${esc}[0m"
  case "${COLORTERM:-}:${TERM:-}" in
    truecolor:*|24bit:*)
      c_accent="${esc}[38;2;253;123;53m" c_ok="${esc}[38;2;61;220;132m"
      c_fail="${esc}[38;2;239;65;54m" c_muted="${esc}[38;2;139;145;158m" ;;
    *256color*)
      c_accent="${esc}[38;5;208m" c_ok="${esc}[38;5;78m" c_fail="${esc}[38;5;203m" c_muted="${esc}[38;5;245m" ;;
    *)
      c_accent="${esc}[33m" c_ok="${esc}[32m" c_fail="${esc}[31m" c_muted="${esc}[90m" ;;
  esac
fi
# The Linux text console lacks most symbols.
if [ "${TERM:-}" = "linux" ]; then
  g_ok="+" g_fail="x" g_next=">" FRAMES='| / - \'
else
  g_ok="✓" g_fail="✗" g_next="›" FRAMES='⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏'
fi

# step MARKER LABEL DETAIL prints one aligned line.
step() {
  printf '  %s %-14s %s%s%s\n' "$1" "$2" "$c_muted" "$3" "$c_reset"
}
ok() { spin_stop; step "${c_ok}${g_ok}${c_reset}" "$1" "$2"; }
# next COMMAND WHAT shows a command to run next.
next() { printf '  %s%s%s %s  %s%s%s\n' "$c_accent" "$g_next" "$c_reset" "$1" "$c_muted" "$2" "$c_reset"; }
fail() {
  spin_stop
  step "${c_fail}${g_fail}${c_reset}" "$1" "$2" >&2
  exit 1
}

SPIN_PID=""
# spin LABEL shows a spinner until the next ok or fail, on terminals only.
spin() {
  [ -n "$UI_TTY" ] || return 0
  (
    trap 'exit 0' TERM
    while :; do
      for frame in $FRAMES; do
        printf '\r  %s%s%s %s…' "$c_accent" "$frame" "$c_reset" "$1"
        sleep 0.08 2>/dev/null || sleep 1
      done
    done
  ) &
  SPIN_PID=$!
}
spin_stop() {
  [ -n "$SPIN_PID" ] || return 0
  kill "$SPIN_PID" 2>/dev/null || true
  wait "$SPIN_PID" 2>/dev/null || true
  SPIN_PID=""
  printf '\r\033[K'
}
tmp=""
trap 'spin_stop; [ -z "$tmp" ] || rm -rf "$tmp"' EXIT
trap 'spin_stop; exit 130' INT TERM

# --- Platform and download ------------------------------------------------

is_command() {
  command -v "$1" >/dev/null 2>&1
}

is_ci() {
  [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ] || [ -n "${GITLAB_CI:-}" ] || [ -n "${CIRCLECI:-}" ] || [ -n "${TRAVIS:-}" ] || [ -n "${JENKINS_URL:-}" ] || [ -n "${BUILDKITE:-}" ]
}

detect_platform() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$OS" in
    darwin) OS_TITLE=Darwin ;;
    linux) OS_TITLE=Linux ;;
    mingw*|msys*|cygwin*|windows*) OS=windows OS_TITLE=Windows ;;
    *) fail "Platform" "$OS is not supported (macOS, Linux and Windows are)" ;;
  esac
  case "$(uname -m)" in
    x86_64|x86-64|x64|amd64) ARCH=x86_64 ;;
    arm64|aarch64|armv8*) ARCH=arm64 ;;
    i386|i686|x86|386) ARCH=i386 ;;
    *) fail "Platform" "$(uname -m) is not supported (x86_64, arm64 and i386 are)" ;;
  esac
}

# download URL FILE
download() {
  if is_command curl; then
    curl --fail --silent --show-error --location --retry 2 -o "$2" "$1"
  elif is_command wget; then
    wget -q -O "$2" "$1"
  else
    fail "Download" "curl or wget is required"
  fi
}

# Tags of releases that are not meant for everyone.
UNSTABLE='preview|alpha|beta|rc|dev|pre|snapshot|nightly|canary|experimental|unstable'

# latest_version prints the newest stable release tag, from the redirect
# GitHub serves for the latest release; the rate-limited API is the fallback.
latest_version() {
  if is_command curl; then
    curl --silent --head "$RELEASES/latest"
  elif is_command wget; then
    # BusyBox wget has no --max-redirect; -S prints every response's headers.
    wget -S --spider "$RELEASES/latest" 2>&1
  fi | tr -d '\r' | sed -n 's#^ *[Ll]ocation: .*/tag/\([^ ]*\).*#\1#p' | head -n 1 > "$tmp/latest" || true
  tag=$(cat "$tmp/latest")
  if [ -n "$tag" ] && ! echo "$tag" | grep -qE "$UNSTABLE"; then
    echo "$tag"
    return
  fi
  download "https://api.github.com/repos/$OWNER/$REPO/releases" "$tmp/releases" 2>/dev/null || return 0
  grep '"tag_name":' "$tmp/releases" | cut -f4 -d'"' | grep -vE "$UNSTABLE" | head -n 1
}

sha256() {
  if is_command sha256sum; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif is_command shasum; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  elif is_command openssl; then
    openssl dgst -sha256 "$1" | sed 's/^.*= //'
  fi
}

# verify FILE CHECKSUMS checks the archive against the release checksums.
verify() {
  want=$(grep " $NAME\$" "$2" 2>/dev/null | cut -d ' ' -f 1)
  [ -n "$want" ] || fail "Checksum" "$NAME is not listed in the release checksums"
  got=$(sha256 "$1")
  # Nothing is installed unverified.
  [ -n "$got" ] || fail "Checksum" "cannot verify the download: install sha256sum, shasum or openssl"
  [ "$got" = "$want" ] || fail "Checksum" "the download does not match the release checksums; try again"
}

display_path() {
  case "$1" in
    "$HOME"/*) echo "~${1#"$HOME"}" ;;
    *) echo "$1" ;;
  esac
}

install_cli() {
  detect_platform
  spin "Installing the Blaxel CLI"
  tmp=$(mktemp -d)
  if [ -z "${VERSION:-}" ] || [ "$VERSION" = "latest" ]; then
    VERSION=$(latest_version)
    [ -n "$VERSION" ] || fail "Blaxel CLI" "could not find the latest release; set VERSION (see $RELEASES)"
  fi
  case "$VERSION" in v*) ;; *) VERSION="v$VERSION" ;; esac
  NAME="${BINARY}_${OS_TITLE}_${ARCH}.tar.gz" EXE=""
  [ "$OS" = "windows" ] && NAME="${BINARY}_${OS_TITLE}_${ARCH}.zip" EXE=".exe"

  download "$RELEASES/download/$VERSION/$NAME" "$tmp/$NAME" ||
    fail "Blaxel CLI" "could not download $VERSION for $OS_TITLE $ARCH"
  download "$RELEASES/download/$VERSION/${BINARY}_${VERSION#v}_checksums.txt" "$tmp/checksums.txt" ||
    fail "Checksum" "could not download the checksums of $VERSION"
  verify "$tmp/$NAME" "$tmp/checksums.txt"
  case "$NAME" in
    *.zip) unzip -q -o "$tmp/$NAME" -d "$tmp" ;;
    *) tar -xzf "$tmp/$NAME" -C "$tmp" ;;
  esac
  mkdir -p "$BINDIR"
  BINDIR=$(cd "$BINDIR" && pwd)
  install "$tmp/$BINARY$EXE" "$BINDIR/$BINARY$EXE"
  install "$tmp/$BINARY$EXE" "$BINDIR/$BINARY_SHORT_NAME$EXE"
  rm -rf "$tmp"
  tmp=""
  fix_owner "$BINDIR/$BINARY$EXE" "$BINDIR/$BINARY_SHORT_NAME$EXE"
  ok "Blaxel CLI" "$VERSION · $(display_path "$BINDIR")/$BINARY_SHORT_NAME · verified"
}

# --- Shell: PATH and completions -------------------------------------------

detect_shell() {
  SHELL_NAME=$(basename "${SHELL:-sh}")
  case "$SHELL_NAME" in
    zsh) RC_FILE="$HOME/.zshrc" ;;
    bash)
      RC_FILE="$HOME/.bashrc"
      if [ -f "$HOME/.bash_profile" ]; then RC_FILE="$HOME/.bash_profile"; fi
      ;;
    fish) RC_FILE="$HOME/.config/fish/config.fish" ;;
    *) SHELL_NAME=sh RC_FILE="$HOME/.profile" ;;
  esac
}

# rc_has_path reports whether the RC file already puts BINDIR on PATH.
rc_has_path() {
  [ -f "$RC_FILE" ] || return 1
  awk -v shell_name="$SHELL_NAME" -v bin_path="$BINDIR" '
    /^[[:space:]]*#/ { next }
    shell_name == "fish" && /^[[:space:]]*(set[[:space:]]+-gx[[:space:]]+PATH|fish_add_path)[[:space:]]+/ && index($0, bin_path) > 0 { found = 1 }
    shell_name != "fish" && /^[[:space:]]*(export[[:space:]]+)?PATH=/ && index($0, bin_path) > 0 { found = 1 }
    END { exit found ? 0 : 1 }
  ' "$RC_FILE"
}

# setup_path adds BINDIR to PATH in the shell's RC file. CI leaves it alone
# unless BL_INSTALL_PATH=true.
setup_path() {
  case ":$PATH:" in *":$BINDIR:"*) SHELL_DONE="bl on PATH"; return ;; esac
  if rc_has_path; then
    SHELL_DONE="bl on PATH" RELOAD="source $(display_path "$RC_FILE")"
    return
  fi
  if [ "${BL_INSTALL_PATH:-}" = "false" ] || { is_ci && [ "${BL_INSTALL_PATH:-}" != "true" ]; }; then
    RELOAD="export PATH=\"$(display_path "$BINDIR"):\$PATH\""
    return
  fi
  mkdir -p "$(dirname "$RC_FILE")"
  if [ "$SHELL_NAME" = "fish" ]; then
    printf '\n# Added by the Blaxel installer\nset -gx PATH %s $PATH\n' "$BINDIR" >> "$RC_FILE"
  else
    printf '\n# Added by the Blaxel installer\nexport PATH="%s:$PATH"\n' "$BINDIR" >> "$RC_FILE"
  fi
  SHELL_DONE="bl on PATH ($(display_path "$RC_FILE"))" RELOAD="source $(display_path "$RC_FILE")"
}

# setup_completion installs completions where the shell loads them.
setup_completion() {
  if [ "${BL_INSTALL_COMPLETION:-}" = "false" ] || { is_ci && [ "${BL_INSTALL_COMPLETION:-}" != "true" ]; }; then
    return
  fi
  case "$SHELL_NAME" in
    zsh) dir="${ZSH_COMPLETION_DIR:-$HOME/.zsh/completions}" file="$dir/_$BINARY_SHORT_NAME" ;;
    bash) dir="${BASH_COMPLETION_USER_DIR:-$HOME/.local/share/bash-completion/completions}" file="$dir/$BINARY_SHORT_NAME" ;;
    fish) dir="$HOME/.config/fish/completions" file="$dir/$BINARY_SHORT_NAME.fish" ;;
    *) return ;;
  esac
  mkdir -p "$dir"
  if ! "$BINDIR/$BINARY_SHORT_NAME" completion "$SHELL_NAME" > "$file.tmp" 2>/dev/null; then
    rm -f "$file.tmp"
    return
  fi
  if [ "$SHELL_NAME" = "bash" ]; then
    # macOS bash 3.2 has no bash-completion package; define the one helper it needs.
    {
      cat <<'BASH_SHIM'
# Shim: provide _get_comp_words_by_ref if bash-completion is not installed.
if ! type _get_comp_words_by_ref >/dev/null 2>&1; then
    _get_comp_words_by_ref() {
        local exclude cur_ words_ cword_
        if [ "$1" = "-n" ]; then
            exclude=$2
            shift 2
        fi
        while [ $# -gt 0 ]; do
            case "$1" in
                cur)   cur="${COMP_WORDS[COMP_CWORD]}" ;;
                prev)  prev="${COMP_WORDS[COMP_CWORD-1]}" ;;
                words) eval words='("${COMP_WORDS[@]}")' ;;
                cword) cword=$COMP_CWORD ;;
            esac
            shift
        done
    }
fi

BASH_SHIM
      cat "$file.tmp"
    } > "$file"
    rm -f "$file.tmp"
  else
    mv "$file.tmp" "$file"
  fi
  if [ "$SHELL_NAME" = "zsh" ] && ! grep -q "$dir" "$RC_FILE" 2>/dev/null; then
    printf '\n# Added by the Blaxel installer: completions\nfpath=(%s $fpath)\nautoload -Uz compinit && compinit\n' "$dir" >> "$RC_FILE"
  fi
  fix_owner "$file"
  SHELL_DONE="${SHELL_DONE:+$SHELL_DONE · }$SHELL_NAME completions"
}

# fix_owner PATH... gives what the installer wrote in the sudo user's home, and
# the folders it made for it, back to that user. Nothing outside it changes.
fix_owner() {
  [ -n "$SUDO_HOME" ] || return 0
  for path in "$@"; do
    while :; do
      case "$path" in "$SUDO_HOME"/*) ;; *) break ;; esac
      if [ -e "$path" ] && [ -O "$path" ]; then
        chown "$SUDO_USER:$(id -g "$SUDO_USER")" "$path" 2>/dev/null || true
      fi
      path=$(dirname "$path")
    done
  done
}

# --- Hand-off to bl setup ---------------------------------------------------

# setup_mode decides how bl setup runs: "interactive" on a terminal, "yes"
# with the defaults when BL_INSTALL_SETUP=true, or "" to only print the next step.
setup_mode() {
  SETUP_MODE=""
  [ "${BL_INSTALL_SETUP:-}" != "false" ] || return 0
  # Releases before bl setup existed (VERSION=...) leave it to the user.
  "$BINDIR/$BINARY" setup --help >/dev/null 2>&1 || return 0
  forced=""
  if [ "${BL_INSTALL_SETUP:-}" = "true" ] || [ "${BL_INSTALL_SKILLS:-}" = "true" ]; then
    forced=1
  fi
  [ -n "$forced" ] || ! is_ci || return 0
  if [ -t 1 ] && [ -e /dev/tty ] && ( : < /dev/tty ) 2>/dev/null; then
    SETUP_MODE=interactive
  elif [ -n "$forced" ]; then
    SETUP_MODE=yes
  fi
}

# run_setup hands the terminal to bl setup, which shows what it found and
# installs it. As root (sudo), it sets up the real user's agents and login;
# the command goes through stdin so sudo's login shell does not expand it.
run_setup() {
  args="setup" input="/dev/tty"
  [ "$SETUP_MODE" = "yes" ] && args="setup --yes" input="/dev/null"
  runner=""
  if [ "$(id -u)" = "0" ] && [ -n "${SUDO_USER:-}" ] && is_command sudo; then
    runner="sudo -u $SUDO_USER -H -i"
  fi
  quote() { printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"; }
  printf 'exec env BL_INSTALLER=1 BL_INSTALLER_SHELL=%s BL_INSTALLER_RELOAD=%s %s %s < %s\n' \
    "$(quote "${SHELL_DONE:-}")" "$(quote "${RELOAD:-}")" "$(quote "$BINDIR/$BINARY")" "$args" "$input" | $runner sh -s
}

main() {
  case "${1:-}" in
    -h|--help) sed -n '2,17p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//'; exit 0 ;;
  esac
  echo
  install_cli
  detect_shell
  SHELL_DONE="" RELOAD=""
  setup_path
  setup_completion
  fix_owner "$RC_FILE"
  setup_mode
  # bl setup shows the shell and what to run next itself, problems included.
  if [ -n "$SETUP_MODE" ]; then
    run_setup || true
    return 0
  fi
  [ -n "$SHELL_DONE" ] && ok "Shell" "$SHELL_DONE"
  bl="$BINARY_SHORT_NAME"
  case ":$PATH:" in *":$BINDIR:"*) ;; *) bl="$(display_path "$BINDIR")/$BINARY_SHORT_NAME" ;; esac
  echo
  [ -n "$RELOAD" ] && next "$RELOAD" "use bl in this terminal"
  # Skipping bl setup on purpose (as bl upgrade does) needs no reminder.
  if [ "${BL_INSTALL_SETUP:-}" = "false" ]; then
    :
  elif "$BINDIR/$BINARY" setup --help >/dev/null 2>&1; then
    next "$bl setup" "set up your coding agents and log in"
  else
    next "$bl login" "log in to Blaxel"
  fi
  return 0
}

main "$@"
