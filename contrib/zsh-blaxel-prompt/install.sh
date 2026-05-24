#!/bin/sh
set -eu

plugin_dir="${BLAXEL_PROMPT_PLUGIN_DIR:-${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/zsh-blaxel-prompt}"
source_dir="${BLAXEL_PROMPT_INSTALL_SOURCE_DIR:-}"
base_url="${BLAXEL_PROMPT_INSTALL_BASE_URL:-https://raw.githubusercontent.com/blaxel-ai/toolkit/main/contrib/zsh-blaxel-prompt}"

mkdir -p "$plugin_dir"

install_file() {
  name="$1"
  mode="$2"
  target="$plugin_dir/$name"

  if [ -n "$source_dir" ]; then
    cp "$source_dir/$name" "$target"
  elif command -v curl >/dev/null 2>&1; then
    curl -fsSL "$base_url/$name" -o "$target"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$base_url/$name" -O "$target"
  else
    echo "curl or wget is required to install zsh-blaxel-prompt" >&2
    exit 1
  fi

  chmod "$mode" "$target"
}

install_file "zsh-blaxel-prompt.plugin.zsh" 0644
install_file "blaxel.zsh" 0644
install_file "blaxel-workspace" 0755

cat <<EOF
zsh-blaxel-prompt installed to:
  $plugin_dir

Enable it in ~/.zshrc:
  plugins=(... zsh-blaxel-prompt)

For quick prompt insertion, set before Oh My Zsh loads:
  BLAXEL_PROMPT_AUTO=prepend

For exact placement, add this to your theme:
  \$(blaxel_prompt_info)
EOF
