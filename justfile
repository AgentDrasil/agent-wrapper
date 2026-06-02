# Format code with goimports
fmt:
    goimports -w -local "github.com/AgentDrasil/agent-wrapper" .

# Run golangci-lint
lint:
    golangci-lint run

# Install aw and agystatusline binaries
install:
    go install ./cmd/aw
    go install ./cmd/agystatusline

