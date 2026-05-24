setopt prompt_subst

autoload -U add-zsh-hook

BLAXEL_PROMPT_PREFIX="${BLAXEL_PROMPT_PREFIX:-on }"
BLAXEL_PROMPT_SYMBOL="${BLAXEL_PROMPT_SYMBOL:-🏀 }"
BLAXEL_PROMPT_SUFFIX="${BLAXEL_PROMPT_SUFFIX:- }"
BLAXEL_PROMPT_AUTO="${BLAXEL_PROMPT_AUTO:-}"

function _blaxel_prompt_plugin_dir() {
  print -r -- "${${(%):-%x}:A:h}"
}

function blaxel_current_workspace() {
  local helper config_file workspace

  if [[ -n "$BL_WORKSPACE" ]]; then
    print -r -- "$BL_WORKSPACE"
    return 0
  fi

  helper="$(_blaxel_prompt_plugin_dir)/blaxel-workspace"
  if [[ -x "$helper" ]]; then
    workspace="$("$helper" 2>/dev/null)" || return 1
    [[ -n "$workspace" ]] || return 1
    print -r -- "$workspace"
    return 0
  fi

  config_file="$HOME/.blaxel/config.yaml"
  [[ -f "$config_file" ]] || return 1

  workspace="$(
    awk '
      /^[[:space:]]*#/ { next }
      /^[^[:space:]][^:]*:/ {
        in_context = ($0 ~ /^context:[[:space:]]*$/)
        next
      }
      in_context && /^[[:space:]]+workspace:[[:space:]]*/ {
        sub(/^[[:space:]]*workspace:[[:space:]]*/, "")
        sub(/[[:space:]]+#.*$/, "")
        gsub(/^[[:space:]]+|[[:space:]]+$/, "")
        gsub(/^["'\''"]|["'\''"]$/, "")
        print
        exit
      }
    ' "$config_file"
  )"

  [[ -n "$workspace" ]] || return 1
  print -r -- "$workspace"
}

function blaxel_prompt_info() {
  local workspace

  workspace="$(blaxel_current_workspace)" || return 1
  [[ -n "$workspace" ]] || return 1
  workspace="${workspace//\%/%%}"

  print -rn -- "${BLAXEL_PROMPT_PREFIX}${BLAXEL_PROMPT_SYMBOL}${workspace}${BLAXEL_PROMPT_SUFFIX}"
}

function _blaxel_prompt_segment_ref() {
  print -r -- '$(blaxel_prompt_info)'
}

function blaxel_prompt_prepend() {
  local segment

  segment="$(_blaxel_prompt_segment_ref)"
  [[ "$PROMPT" == *"$segment"* ]] && return 0
  PROMPT="${segment}${PROMPT}"
}

function blaxel_prompt_append() {
  local segment

  segment="$(_blaxel_prompt_segment_ref)"
  [[ "$PROMPT" == *"$segment"* ]] && return 0
  PROMPT="${PROMPT}${segment}"
}

function _blaxel_prompt_auto_apply() {
  add-zsh-hook -d precmd _blaxel_prompt_auto_apply 2>/dev/null

  case "$BLAXEL_PROMPT_AUTO" in
    prepend|prefix|left)
      blaxel_prompt_prepend
      ;;
    append|suffix)
      blaxel_prompt_append
      ;;
  esac
}

if [[ -n "$BLAXEL_PROMPT_AUTO" ]]; then
  add-zsh-hook precmd _blaxel_prompt_auto_apply
fi

# Backward-compatible variables for users who source this plugin directly.
function _zsh_blaxel_prompt_precmd() {
  local workspace

  workspace="$(blaxel_current_workspace)" || {
    ZSH_BLAXEL_WORKSPACE=""
    ZSH_BLAXEL_PROMPT=""
    return 1
  }

  ZSH_BLAXEL_WORKSPACE="$workspace"
  ZSH_BLAXEL_PROMPT="$(blaxel_prompt_info)"
}

add-zsh-hook precmd _zsh_blaxel_prompt_precmd
