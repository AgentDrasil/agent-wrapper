# Format code with goimports
fmt:
    goimports -w -local "github.com/AgentDrasil/agent-wrapper" .

# Run golangci-lint
lint:
    golangci-lint run
