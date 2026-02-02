# Code Server Python Image for Sandbox Engine
# Specialized for Python/FastAPI development

FROM ghcr.io/terra-clan/sandbox-engine/code-server:latest

USER root

# Python специфичные инструменты
RUN pip3 install --no-cache-dir --break-system-packages \
    poetry \
    pipenv \
    black \
    flake8 \
    mypy \
    pytest \
    fastapi \
    uvicorn \
    sqlalchemy \
    alembic \
    pydantic

USER coder

# Python extensions
RUN code-server --install-extension ms-python.python \
    && code-server --install-extension ms-python.vscode-pylance
