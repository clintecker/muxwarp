# fish completion for muxwarp

complete -c muxwarp -l help -d 'Show help'
complete -c muxwarp -l version -d 'Print version'
complete -c muxwarp -l log -r -F -d 'Write debug logs to file'
complete -c muxwarp -l completions -r -f -a 'bash zsh fish' -d 'Output shell completions'

complete -c muxwarp -n '__fish_use_subcommand' -a init -d 'Generate config from ~/.ssh/config'
complete -c muxwarp -n '__fish_seen_subcommand_from init' -l force -d 'Overwrite existing config'
