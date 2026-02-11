#!/bin/bash
# Run install.sh end-to-end tests inside Docker.
#
# Usage:
#   ./test/install/run_tests.sh           Run tests and exit
#   ./test/install/run_tests.sh --shell   Run tests, then drop into the container
#                                         so you can manually test completions:
#                                           bash:  type "bl " then TAB
#                                           zsh:   run "zsh", type "bl " then TAB
#                                           fish:  run "fish", type "bl " then TAB
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

INTERACTIVE=false
if [ "${1:-}" = "--shell" ] || [ "${1:-}" = "-i" ]; then
  INTERACTIVE=true
fi

echo "Building test Docker image..."
docker build \
  -f "$SCRIPT_DIR/Dockerfile" \
  -t blaxel-install-test \
  "$PROJECT_ROOT"

echo ""
echo "Running install tests..."
docker run --rm blaxel-install-test

if [ "$INTERACTIVE" = true ]; then
  echo ""
  echo "Starting interactive container..."
  docker run --rm -it \
    --entrypoint /bin/bash \
    blaxel-install-test \
    -c '
      # Install with all options enabled, no prompts
      export BL_INSTALL_PATH=true
      export BL_INSTALL_COMPLETION=true
      export BL_INSTALL_TRACKING=true
      SHELL=/bin/bash sh /home/testuser/install.sh
      export PATH="$HOME/.local/bin:$PATH"

      # Generate completions for zsh and fish (bash was already set up by install.sh with shim)
      mkdir -p "$HOME/.zsh/completions" "$HOME/.config/fish/completions"
      bl completion zsh > "$HOME/.zsh/completions/_bl" 2>/dev/null || true
      bl completion fish > "$HOME/.config/fish/completions/bl.fish" 2>/dev/null || true

      # Set up zsh fpath
      printf "\nfpath=(%s/.zsh/completions \$fpath)\nautoload -Uz compinit && compinit\n" "$HOME" >> "$HOME/.zshrc"

      # Source bash completions in .bashrc
      echo "source $HOME/.local/share/bash-completion/completions/bl 2>/dev/null" >> "$HOME/.bashrc"

      echo ""
      echo "Ready! Completions installed for bash, zsh, and fish."
      echo "  bash:  type \"bl \" then TAB"
      echo "  zsh:   run \"zsh\", type \"bl \" then TAB"
      echo "  fish:  run \"fish\", type \"bl \" then TAB"
      echo ""
      exec bash --rcfile "$HOME/.bashrc"
    '
fi
