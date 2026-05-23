setopt prompt_subst
autoload -U add-zsh-hook

function() {
  local separator

  # Specify the separator between workspace and environment
  zstyle -s ':zsh-blaxel-prompt:' separator separator
  if [[ -z "$separator" ]]; then
    zstyle ':zsh-blaxel-prompt:' separator '/'
  fi
}

add-zsh-hook precmd _zsh_blaxel_prompt_precmd
function _zsh_blaxel_prompt_precmd() {
  local config_file workspace env separator preprompt postprompt
  local modified_time_fmt now updated_at

  config_file="$HOME/.blaxel/config.yaml"

  # If BL_WORKSPACE is set, use it directly
  if [[ -n "$BL_WORKSPACE" ]]; then
    ZSH_BLAXEL_WORKSPACE="$BL_WORKSPACE"
    ZSH_BLAXEL_PROMPT="$BL_WORKSPACE"
    return 0
  fi

  # Check if config file exists
  if [[ ! -f "$config_file" ]]; then
    ZSH_BLAXEL_WORKSPACE=""
    ZSH_BLAXEL_PROMPT="no config"
    return 1
  fi

  # Determine the stat format for modification time
  zstyle -s ':zsh-blaxel-prompt:' modified_time_fmt modified_time_fmt
  if [[ -z "$modified_time_fmt" ]]; then
    if stat --help >/dev/null 2>&1; then
      modified_time_fmt='-c%y' # GNU coreutils
    else
      modified_time_fmt='-f%m' # FreeBSD/macOS
    fi
    zstyle ':zsh-blaxel-prompt:' modified_time_fmt "$modified_time_fmt"
  fi

  # Check if config file has changed since last read
  now="$(stat -L $modified_time_fmt "$config_file" 2>/dev/null)"
  if [[ $? -ne 0 ]]; then
    ZSH_BLAXEL_WORKSPACE=""
    ZSH_BLAXEL_PROMPT="config error"
    return 1
  fi

  zstyle -s ':zsh-blaxel-prompt:' updated_at updated_at
  if [[ "$updated_at" == "$now" ]]; then
    return 0
  fi
  zstyle ':zsh-blaxel-prompt:' updated_at "$now"

  # Parse workspace from config.yaml
  # Uses simple grep/sed to avoid requiring external YAML parsers
  workspace="$(grep -A1 '^context:' "$config_file" 2>/dev/null | grep 'workspace:' | sed 's/.*workspace:[[:space:]]*//' | sed 's/[[:space:]]*$//' | sed 's/^["'"'"']\(.*\)["'"'"']$/\1/')"

  if [[ -z "$workspace" ]]; then
    ZSH_BLAXEL_WORKSPACE=""
    ZSH_BLAXEL_PROMPT="no workspace"
    return 1
  fi

  ZSH_BLAXEL_WORKSPACE="$workspace"

  # Build the prompt string
  zstyle -s ':zsh-blaxel-prompt:' preprompt preprompt
  zstyle -s ':zsh-blaxel-prompt:' postprompt postprompt
  zstyle -s ':zsh-blaxel-prompt:' separator separator

  ZSH_BLAXEL_PROMPT="${preprompt}${workspace}${postprompt}"

  return 0
}
