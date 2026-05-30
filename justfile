export PKG_CONFIG_PATH := justfile_directory() + "/third_party/ghostty"

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
    bazel clean

# Format code with goimports
fmt:
    goimports -w -local "github.com/AgentDrasil/agent-wrapper" .

# Run golangci-lint
lint: build
    golangci-lint run
