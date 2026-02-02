# Code Server Node.js Image for Sandbox Engine
# Specialized for Node.js/TypeScript development

FROM ghcr.io/terra-clan/sandbox-engine/code-server:latest

USER root

# Node специфичные инструменты
RUN npm install -g \
    typescript \
    ts-node \
    eslint \
    prettier \
    jest \
    pnpm \
    yarn

USER coder

# Node extensions
RUN code-server --install-extension dbaeumer.vscode-eslint \
    && code-server --install-extension esbenp.prettier-vscode
