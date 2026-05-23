# zsh-blaxel-prompt

Display the current Blaxel workspace in your zsh prompt.

![Example](https://uploads.linear.app/d7de25fb-1674-4125-b664-3cc3bb02df7a/b902a912-c42c-44d5-9c72-112d60864d7a/a17fb5b8-961f-480c-9cb8-fdfd1b6ccaf5)

## Usage

Source the plugin from your `~/.zshrc` and configure your prompt:

```sh
autoload -U colors; colors
source /path/to/zsh-blaxel-prompt/blaxel.zsh
RPROMPT='%{$fg[blue]%}($ZSH_BLAXEL_PROMPT)%{$reset_color%}'
```

### Variables

The plugin exposes the following variables:

| Variable | Description |
|----------|-------------|
| `ZSH_BLAXEL_WORKSPACE` | The current workspace name |
| `ZSH_BLAXEL_PROMPT` | Formatted prompt string (with pre/post decorators) |

### Priority

The workspace is resolved in this order:

1. `BL_WORKSPACE` environment variable (if set)
2. `context.workspace` from `~/.blaxel/config.yaml`

## Installation

### Manual

Clone this repository and source `blaxel.zsh` from your `~/.zshrc`:

```sh
git clone https://github.com/blaxel-ai/toolkit.git
source /path/to/toolkit/contrib/zsh-blaxel-prompt/blaxel.zsh
```

### Oh My Zsh

1. Clone into your custom plugins directory:

```sh
git clone https://github.com/blaxel-ai/toolkit.git
mkdir -p ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-blaxel-prompt
cp toolkit/contrib/zsh-blaxel-prompt/blaxel.zsh ${ZSH_CUSTOM:-~/.oh-my-zsh/custom}/plugins/zsh-blaxel-prompt/zsh-blaxel-prompt.plugin.zsh
```

2. Add `zsh-blaxel-prompt` to your plugins in `~/.zshrc`:

```sh
plugins=( [plugins...] zsh-blaxel-prompt)
```

3. Configure your prompt:

```sh
RPROMPT='%{$fg[blue]%}($ZSH_BLAXEL_PROMPT)%{$reset_color%}'
```

### Antigen

```sh
antigen bundle blaxel-ai/toolkit path:contrib/zsh-blaxel-prompt
```

### Zgen

```sh
zgen load blaxel-ai/toolkit contrib/zsh-blaxel-prompt
```

## Customization

### Pre/Post prompt decorators

Add characters before or after the prompt:

```sh
zstyle ':zsh-blaxel-prompt:' preprompt 'bl:'
zstyle ':zsh-blaxel-prompt:' postprompt ''
```

### Example with color based on workspace name

```sh
autoload -U colors; colors
source /path/to/zsh-blaxel-prompt/blaxel.zsh

function blaxel_prompt() {
  local color="blue"

  if [[ "$ZSH_BLAXEL_WORKSPACE" =~ "prod" ]]; then
    color=red
  elif [[ "$ZSH_BLAXEL_WORKSPACE" =~ "staging" ]]; then
    color=yellow
  fi

  echo "%{$fg[$color]%}(bl:$ZSH_BLAXEL_PROMPT)%{$reset_color%}"
}

RPROMPT='$(blaxel_prompt)'
```

## Performance

The plugin only re-reads the config file when its modification time changes, so it has minimal impact on prompt rendering performance.

## License

MIT
