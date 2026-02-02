# Code Server Base Image for Sandbox Engine
# VS Code in browser with development tools

FROM codercom/code-server:latest

USER root

# Базовые инструменты
RUN apt-get update && apt-get install -y \
    git \
    curl \
    wget \
    vim \
    nano \
    htop \
    jq \
    tree \
    unzip \
    postgresql-client \
    redis-tools \
    && rm -rf /var/lib/apt/lists/*

# Python
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Node.js 20
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Go
RUN wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz \
    && rm go1.22.0.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin

# Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Рабочая директория
WORKDIR /home/coder/workspace

# Права для coder пользователя
RUN chown -R coder:coder /home/coder

USER coder

# Предустановленные extensions
RUN code-server --install-extension ms-python.python \
    && code-server --install-extension golang.go \
    && code-server --install-extension dbaeumer.vscode-eslint \
    && code-server --install-extension esbenp.prettier-vscode

# code-server config
RUN mkdir -p /home/coder/.config/code-server
COPY config.yaml /home/coder/.config/code-server/config.yaml

EXPOSE 8080

ENTRYPOINT ["/usr/bin/entrypoint.sh", "--bind-addr", "0.0.0.0:8080", "--auth", "none", "/home/coder/workspace"]
