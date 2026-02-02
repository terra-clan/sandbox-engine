# Code Server Go Image for Sandbox Engine
# Specialized for Go development

FROM ghcr.io/terra-clan/sandbox-engine/code-server:latest

USER root

# Go tools
RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

ENV PATH=$PATH:/root/go/bin

USER coder

# Go extensions
RUN code-server --install-extension golang.go
