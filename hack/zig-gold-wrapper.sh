#!/bin/bash
# Wrapper script to make zig appear as gold linker to Go

args=()
for arg in "$@"; do
    case "$arg" in
        --version|-v|-Wl,--version)
            # If being asked for version info, pretend to be gold
            echo "GNU gold (GNU Binutils 2.40) 1.16"
            exit 0
            ;;
        -fuse-ld=gold)
            # Go explicitly requests gold; skip this flag as Zig uses LLD
            ;;
        -plugin=*|-plugin|-plugin-opt=*|-plugin-opt|-Wl,-plugin*|-Wl,--plugin*)
            # Skip gold linker plugin flags
            ;;
        *)
            args+=("$arg")
            ;;
    esac
done

# Use zig cc as the linker
exec zig cc "${args[@]}"
