# Update git submodules recursively
submodule-update:
    git submodule update --init --recursive

# Regenerate BUILD files after go.mod changes
gomod:
    bazel run //:gazelle

# Build the project
build:
    bazel build //:agent_wrapper

# Clean all build artifacts, exported libraries, and Go build cache
clean:
    @echo "Cleaning build artifacts..."
    rm -rf out agent-wrapper .zig-cache
    go clean -cache
