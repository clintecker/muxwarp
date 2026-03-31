# bash completion for muxwarp

_muxwarp() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "${prev}" in
        --log)
            COMPREPLY=( $(compgen -f -- "${cur}") )
            return 0
            ;;
        --completions)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
            return 0
            ;;
        init)
            COMPREPLY=( $(compgen -W "--force" -- "${cur}") )
            return 0
            ;;
    esac

    if [[ "${cur}" == -* ]]; then
        opts="-h --help --version --log --completions"
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi

    COMPREPLY=( $(compgen -W "init" -- "${cur}") )
    return 0
}

complete -F _muxwarp muxwarp
