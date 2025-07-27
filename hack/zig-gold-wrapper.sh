#!/bin/bash
# Wrapper script to make zig appear as gold linker to Go

# If being asked for version info, pretend to be gold
if [[ "$*" == *"--version"* ]] || [[ "$*" == *"-v"* ]]; then
    echo "GNU gold (GNU Binutils 2.40) 1.16"
    exit 0
fi

# Use zig cc as the linker
exec zig cc "$@"
