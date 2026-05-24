#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

mkdir -p "$TMP_HOME/.blaxel"
cat > "$TMP_HOME/.blaxel/config.yaml" <<'YAML'
context:
  workspace: config-ws
workspaces:
tracking: false
YAML

RESULT="$(
  HOME="$TMP_HOME" zsh -fc '
    source "'"$SCRIPT_DIR"'/zsh-blaxel-prompt.plugin.zsh"

    print -- "workspace=$(blaxel_current_workspace)"
    print -- "prompt=$(blaxel_prompt_info)"

    BL_WORKSPACE=env-ws
    print -- "env_workspace=$(blaxel_current_workspace)"
    print -- "env_prompt=$(blaxel_prompt_info)"

    unset BL_WORKSPACE
    BLAXEL_PROMPT_PREFIX="with "
    BLAXEL_PROMPT_SYMBOL="🏀 "
    BLAXEL_PROMPT_SUFFIX=""
    print -- "custom=$(blaxel_prompt_info)"

    PROMPT="❯ "
    blaxel_prompt_prepend
    print -- "prepend=$PROMPT"
    blaxel_prompt_prepend
    print -- "prepend_idempotent=$PROMPT"

    PROMPT="❯ "
    blaxel_prompt_append
    print -- "append=$PROMPT"
  '
)"

grep -qx 'workspace=config-ws' <<<"$RESULT"
grep -qx 'prompt=on 🏀 config-ws ' <<<"$RESULT"
grep -qx 'env_workspace=env-ws' <<<"$RESULT"
grep -qx 'env_prompt=on 🏀 env-ws ' <<<"$RESULT"
grep -qx 'custom=with 🏀 config-ws' <<<"$RESULT"
grep -qx 'prepend=$(blaxel_prompt_info)❯ ' <<<"$RESULT"
grep -qx 'prepend_idempotent=$(blaxel_prompt_info)❯ ' <<<"$RESULT"
grep -qx 'append=❯ $(blaxel_prompt_info)' <<<"$RESULT"

HELPER_RESULT="$(HOME="$TMP_HOME" "$SCRIPT_DIR/blaxel-workspace")"
[ "$HELPER_RESULT" = "config-ws" ]

HELPER_ENV_RESULT="$(HOME="$TMP_HOME" BL_WORKSPACE=env-ws "$SCRIPT_DIR/blaxel-workspace")"
[ "$HELPER_ENV_RESULT" = "env-ws" ]

INSTALL_HOME="$TMP_HOME/install-home"
INSTALL_CUSTOM="$INSTALL_HOME/.oh-my-zsh/custom"
mkdir -p "$INSTALL_CUSTOM"
HOME="$INSTALL_HOME" ZSH_CUSTOM="$INSTALL_CUSTOM" BLAXEL_PROMPT_INSTALL_SOURCE_DIR="$SCRIPT_DIR" sh "$SCRIPT_DIR/install.sh" >/dev/null
test -f "$INSTALL_CUSTOM/plugins/zsh-blaxel-prompt/zsh-blaxel-prompt.plugin.zsh"
test -f "$INSTALL_CUSTOM/plugins/zsh-blaxel-prompt/blaxel.zsh"
test -x "$INSTALL_CUSTOM/plugins/zsh-blaxel-prompt/blaxel-workspace"

echo "zsh-blaxel-prompt tests passed"
