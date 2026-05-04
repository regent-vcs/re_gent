# Shell Completion

re_gent supports shell completion for bash, zsh, and fish. This enables tab-completion for commands, flags, and arguments.

## Automatic Installation (Homebrew)

If you installed via Homebrew, shell completion is **automatically set up** for you:

```bash
brew tap regent-vcs/tap
brew install regent
```

No additional configuration needed! Restart your shell or source your profile to enable completions.

## Manual Installation

If you installed via `go install` or from source, follow the instructions for your shell:

### Bash

**Linux:**
```bash
rgt completion bash > /etc/bash_completion.d/rgt
```

**macOS (with bash-completion from Homebrew):**
```bash
rgt completion bash > $(brew --prefix)/etc/bash_completion.d/rgt
```

**Without package manager:**
```bash
rgt completion bash > ~/.bash_completion
echo 'source ~/.bash_completion' >> ~/.bashrc
```

### Zsh

```bash
rgt completion zsh > "${fpath[1]}/_rgt"
```

Or add to your `.zshrc`:
```bash
source <(rgt completion zsh)
```

**Note:** For zsh completion to work, you may need to add this to your `.zshrc` before sourcing completion:
```bash
autoload -U compinit
compinit
```

### Fish

```bash
rgt completion fish > ~/.config/fish/completions/rgt.fish
```

Or add to your `config.fish`:
```bash
rgt completion fish | source
```

## Testing

After installation, restart your shell or source your profile:

```bash
# Bash
source ~/.bashrc

# Zsh
source ~/.zshrc

# Fish
source ~/.config/fish/config.fish
```

Test completion by typing:
```bash
rgt <TAB>
```

You should see available commands like `init`, `log`, `status`, `blame`, etc.

## Completion Features

The shell completion provides:

- **Command completion**: `rgt l<TAB>` → `rgt log`
- **Flag completion**: `rgt log --<TAB>` → shows all available flags
- **Subcommand completion**: `rgt <TAB>` → lists all commands
- **Help text**: Many shells show brief descriptions alongside completions

## Troubleshooting

### Bash completion not working

1. Ensure `bash-completion` is installed:
   ```bash
   # macOS
   brew install bash-completion
   
   # Ubuntu/Debian
   apt-get install bash-completion
   ```

2. Make sure it's sourced in your `.bashrc`:
   ```bash
   [[ -r "/usr/local/etc/profile.d/bash_completion.sh" ]] && . "/usr/local/etc/profile.d/bash_completion.sh"
   ```

### Zsh completion not working

1. Ensure completion system is initialized in your `.zshrc` (should be near the top):
   ```bash
   autoload -U compinit
   compinit
   ```

2. Check that the completion file is in your `fpath`:
   ```bash
   echo $fpath
   ```

3. If needed, add the completion directory to your fpath in `.zshrc`:
   ```bash
   fpath=(~/.zsh/completions $fpath)
   ```

### Fish completion not working

1. Verify the completions directory exists:
   ```bash
   mkdir -p ~/.config/fish/completions
   ```

2. Restart fish or reload config:
   ```bash
   source ~/.config/fish/config.fish
   ```

## Additional Resources

- [Cobra Shell Completions](https://github.com/spf13/cobra/blob/main/site/content/completions/_index.md)
- [Bash Completion Guide](https://github.com/scop/bash-completion)
- [Zsh Completion System](https://zsh.sourceforge.io/Doc/Release/Completion-System.html)
- [Fish Completion Guide](https://fishshell.com/docs/current/completions.html)
