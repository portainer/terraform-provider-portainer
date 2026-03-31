# .devcontainer/Dockerfile
FROM mcr.microsoft.com/devcontainers/go:1.25

RUN apt-get update && apt-get install -y --no-install-recommends unzip curl make git \
    && rm -rf /var/lib/apt/lists/*

USER vscode
