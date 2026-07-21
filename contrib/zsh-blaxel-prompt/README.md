# zsh-blaxel-prompt

Display the current Blaxel workspace in your shell prompt.

![Example](./example.png)

New to Blaxel? Check out the [Getting Started guide](https://docs.blaxel.ai).

## Oh My Zsh

Install the plugin:

```sh
curl -fsSL https://raw.githubusercontent.com/blaxel-ai/toolkit/main/contrib/zsh-blaxel-prompt/install.sh | sh
```

Enable it in `~/.zshrc`:

```zsh
plugins=(... zsh-blaxel-prompt)
```

### Easy Mode

For a quick setup, add this before Oh My Zsh is loaded:

```zsh
BLAXEL_PROMPT_AUTO=prepend
```

This prepends the Blaxel segment to your existing prompt.

### Theme Placement

For exact placement, put this in your Oh My Zsh theme:

```zsh
$(blaxel_prompt_info)
```

For example:

```zsh
PROMPT='$(blaxel_prompt_info)'$PROMPT
```

## Plain Zsh

Install the files anywhere and source `blaxel.zsh`:

```zsh
source ~/.zsh/zsh-blaxel-prompt/blaxel.zsh
PROMPT='$(blaxel_prompt_info)'$PROMPT
```

You can also use the built-in helpers:

```zsh
blaxel_prompt_prepend
blaxel_prompt_append
```

## Starship

Starship users can use the standalone helper as a custom module.

Install the helper:

```sh
mkdir -p ~/.zsh/zsh-blaxel-prompt
curl -o ~/.zsh/zsh-blaxel-prompt/blaxel-workspace \
  https://raw.githubusercontent.com/blaxel-ai/toolkit/main/contrib/zsh-blaxel-prompt/blaxel-workspace
chmod +x ~/.zsh/zsh-blaxel-prompt/blaxel-workspace
```

Add this module to `~/.config/starship.toml`:

```toml
[custom.blaxel]
command = "~/.zsh/zsh-blaxel-prompt/blaxel-workspace"
when = "test -n \"$BL_WORKSPACE\" || test -f \"$HOME/.blaxel/config.yaml\""
symbol = "🏀 "
style = "blue bold"
format = "on [$symbol$output]($style) "
```

With Starship's default `format = "$all"`, custom modules are included automatically:

```text
toolkit on  branch via 🐹 v1.26.2 on ☁️  (us-east-1) on 🏀 my-workspace
❯
```

If you have a custom Starship `format`, include `$custom` where you want custom modules to appear:

```toml
format = "$directory$git_branch$git_status$golang$aws$gcloud$custom$line_break$character"
```

## Customization

The zsh plugin follows the same pattern as common prompt plugins: it exposes a prompt function and simple variables.

```zsh
BLAXEL_PROMPT_PREFIX="on "
BLAXEL_PROMPT_SYMBOL="🏀 "
BLAXEL_PROMPT_SUFFIX=" "
```

The prompt function returns an empty segment when no Blaxel workspace is configured:

```zsh
blaxel_prompt_info
```

The raw workspace helper is also available:

```zsh
blaxel_current_workspace
```

## Workspace Resolution

The workspace is resolved in this order:

1. `BL_WORKSPACE` environment variable, if set
2. `context.workspace` from `~/.blaxel/config.yaml`

## Performance

The helper reads only `BL_WORKSPACE` or the local Blaxel config file. It does not call the Blaxel API.

## License

MIT
