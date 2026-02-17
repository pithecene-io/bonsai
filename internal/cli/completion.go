package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func completionCommand() *cli.Command {
	return &cli.Command{
		Name:  "completion",
		Usage: "Generate shell completion scripts",
		Subcommands: []*cli.Command{
			{
				Name:  "bash",
				Usage: "Generate bash completion script",
				Action: func(c *cli.Context) error {
					fmt.Print(bashCompletion)
					return nil
				},
			},
			{
				Name:  "zsh",
				Usage: "Generate zsh completion script",
				Action: func(c *cli.Context) error {
					fmt.Print(zshCompletion)
					return nil
				},
			},
			{
				Name:  "fish",
				Usage: "Generate fish completion script",
				Action: func(c *cli.Context) error {
					fmt.Print(fishCompletion)
					return nil
				},
			},
		},
	}
}

const bashCompletion = `# bonsai bash completion
# Add to ~/.bashrc: eval "$(bonsai completion bash)"

_bonsai_completions() {
  local cur prev commands
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  commands="version chat plan implement review skill check list patch migrate hooks completion help"

  case "${prev}" in
    bonsai)
      COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
      return 0
      ;;
    chat)
      COMPREPLY=($(compgen -W "architect implementer planner reviewer patch-architect patcher" -- "${cur}"))
      return 0
      ;;
    check)
      COMPREPLY=($(compgen -W "--bundle --mode --scope --base --fail-fast" -- "${cur}"))
      return 0
      ;;
    skill)
      COMPREPLY=($(compgen -W "--version --scope --base" -- "${cur}"))
      return 0
      ;;
    list)
      COMPREPLY=($(compgen -W "--skills --bundles --roles" -- "${cur}"))
      return 0
      ;;
    hooks)
      COMPREPLY=($(compgen -W "install remove" -- "${cur}"))
      return 0
      ;;
    completion)
      COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
      return 0
      ;;
  esac
}

complete -F _bonsai_completions bonsai
`

const zshCompletion = `# bonsai zsh completion
# Add to ~/.zshrc: eval "$(bonsai completion zsh)"

_bonsai() {
  local -a commands
  commands=(
    'version:Print the bonsai version'
    'chat:Start an interactive AI chat session'
    'plan:Start a planning session'
    'implement:Start an implementation session with governance gating'
    'review:Start a code review session'
    'skill:Run a single governance skill'
    'check:Run governance skills'
    'list:List available skills, bundles, or roles'
    'patch:Three-phase patch surgery'
    'migrate:Scaffold AI governance into a repository'
    'hooks:Manage git hooks'
    'completion:Generate shell completion scripts'
    'help:Show help'
  )

  _arguments \
    '1: :->command' \
    '*::arg:->args'

  case "$state" in
    command)
      _describe 'command' commands
      ;;
    args)
      case "${words[1]}" in
        chat)
          _values 'role' architect implementer planner reviewer patch-architect patcher
          ;;
        hooks)
          _values 'action' install remove
          ;;
        completion)
          _values 'shell' bash zsh fish
          ;;
      esac
      ;;
  esac
}

compdef _bonsai bonsai
`

const fishCompletion = `# bonsai fish completion
# Add to fish config: bonsai completion fish | source

complete -c bonsai -f

complete -c bonsai -n '__fish_use_subcommand' -a version -d 'Print the bonsai version'
complete -c bonsai -n '__fish_use_subcommand' -a chat -d 'Start an interactive AI chat session'
complete -c bonsai -n '__fish_use_subcommand' -a plan -d 'Start a planning session'
complete -c bonsai -n '__fish_use_subcommand' -a implement -d 'Start an implementation session'
complete -c bonsai -n '__fish_use_subcommand' -a review -d 'Start a code review session'
complete -c bonsai -n '__fish_use_subcommand' -a skill -d 'Run a single governance skill'
complete -c bonsai -n '__fish_use_subcommand' -a check -d 'Run governance skills'
complete -c bonsai -n '__fish_use_subcommand' -a list -d 'List skills, bundles, or roles'
complete -c bonsai -n '__fish_use_subcommand' -a patch -d 'Three-phase patch surgery'
complete -c bonsai -n '__fish_use_subcommand' -a migrate -d 'Scaffold AI governance'
complete -c bonsai -n '__fish_use_subcommand' -a hooks -d 'Manage git hooks'
complete -c bonsai -n '__fish_use_subcommand' -a completion -d 'Generate completions'
complete -c bonsai -n '__fish_use_subcommand' -a help -d 'Show help'

complete -c bonsai -n '__fish_seen_subcommand_from chat' -a 'architect implementer planner reviewer patch-architect patcher'
complete -c bonsai -n '__fish_seen_subcommand_from hooks' -a 'install remove'
complete -c bonsai -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`
