#!/bin/bash
# Wrapper script to filter out unsupported flags for zig on macOS

args=()
for arg in "$@"; do
    case "$arg" in
        -Wl,-flat_namespace|-Wl,-bind_at_load)
            # Skip these macOS-specific flags that zig doesn't support
            ;;
        -lresolv)
            # Use built-in resolv library
            ;;
        *)
            args+=("$arg")
            ;;
    esac
done

exec zig cc "${args[@]}"
