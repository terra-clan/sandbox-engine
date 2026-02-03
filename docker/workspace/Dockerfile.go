# Go workspace image for sandbox-engine
# Go 1.23 + Claude Code CLI + common tools

FROM ghcr.io/terra-clan/sandbox-engine/workspace-base:latest

USER root

# Install Go
ENV GO_VERSION=1.23.0
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# Set up Go environment
ENV PATH="/usr/local/go/bin:/home/coder/go/bin:${PATH}"
ENV GOPATH=/home/coder/go
ENV GOPROXY=https://proxy.golang.org,direct

# Create Go directories
RUN mkdir -p /home/coder/go/{bin,src,pkg} \
    && chown -R coder:coder /home/coder/go

# Install common Go tools
RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

USER coder

# Environment
ENV GO_ENV=development
ENV CGO_ENABLED=0

CMD ["/bin/bash"]
