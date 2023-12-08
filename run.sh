#!/usr/bin/env bash

set -o errexit
set -o pipefail


GOFUMPT_CMD="docker run --rm -it -e GOFUMPT_SPLIT_LONG_LINES=on -v $(pwd):/work ghcr.io/hellt/gofumpt:0.3.1"
GOFUMPT_FLAGS="-l -w ."

GODOT_CMD="docker run --rm -it -v $(pwd):/work ghcr.io/hellt/godot:1.4.11"
GODOT_FLAGS="-w ."

function gofumpt {
    ${GOFUMPT_CMD} ${GOFUMPT_FLAGS}
}

function godot {
    ${GODOT_CMD} ${GODOT_FLAGS}
}

function format {
    gofumpt
    godot
}

_run_sh_autocomplete() {
    local current_word
    COMPREPLY=()
    current_word="${COMP_WORDS[COMP_CWORD]}"

    # Get list of function names in run.sh
    local functions=$(declare -F -p | cut -d " " -f 3 | grep -v "^_" | grep -v "nvm_")

    # Generate autocompletions based on the current word
    COMPREPLY=( $(compgen -W "${functions}" -- ${current_word}) )
}

# Specify _run_sh_autocomplete as the source of autocompletions for run.sh
complete -F _run_sh_autocomplete ./run.sh

function help {
  printf "%s <task> [args]\n\nTasks:\n" "${0}"

  compgen -A function | grep -v "^_" | grep -v "nvm_" | cat -n

  printf "\nExtended help:\n  Each task has comments for general usage\n"
}

# This idea is heavily inspired by: https://github.com/adriancooney/Taskfile
TIMEFORMAT=$'\nTask completed in %3lR'
time "${@:-help}"


set -e