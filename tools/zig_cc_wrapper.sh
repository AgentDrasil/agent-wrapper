#!/usr/bin/env bash
# tools/zig_cc_wrapper.sh
#
# Thin shim so that `zig cc` can be invoked as a conventional CC binary.
# rules_go CGO passes the CC path as a single executable; zig requires `zig cc`
# as two tokens, so we wrap it here.
#
# Usage (set in .bazelrc or via --action_env):
#   build --action_env=CC=tools/zig_cc_wrapper.sh
exec zig cc "$@"
