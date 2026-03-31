#compdef muxwarp

_muxwarp() {
    local -a commands flags

    flags=(
        '--help[Show help]'
        '--version[Print version]'
        '--log[Write debug logs]:log file:_files'
        '--completions[Output shell completions]:shell:(bash zsh fish)'
    )

    commands=(
        'init:Generate config from ~/.ssh/config'
    )

    _arguments -s \
        "${flags[@]}" \
        '1:command:->command' \
        '*::arg:->args'

    case "$state" in
        command)
            _describe -t commands 'muxwarp command' commands
            ;;
        args)
            case "${words[1]}" in
                init)
                    _arguments '--force[Overwrite existing config]'
                    ;;
            esac
            ;;
    esac
}

_muxwarp "$@"
