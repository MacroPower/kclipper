FROM golang:1.25

ARG TZ
ENV TZ="$TZ"

WORKDIR /workspace

# Install basic development tools and networking utilities.
RUN apt-get update && apt-get install -y --no-install-recommends \
  curl \
  less \
  man-db \
  gnupg2 \
  iptables \
  ipset \
  iproute2 \
  dnsutils \
  aggregate \
  nano \
  vim \
  xz-utils \
  jq \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Dagger CLI.
RUN curl -fsSL https://dl.dagger.io/dagger/install.sh | BIN_DIR=/usr/local/bin sh

# Install Task runner.
RUN go install github.com/go-task/task/v3/cmd/task@latest

# Install conform (commit message linter).
RUN go install github.com/siderolabs/conform/cmd/conform@latest

# Install lefthook (git hooks manager).
RUN go install github.com/evilmartians/lefthook@latest

# Create non-root developer user.
RUN useradd -m -s /bin/bash developer \
  && mkdir -p /workspace && chown developer:developer /workspace

# Map host home path into the container so mounted configs with absolute
# host paths (e.g. plugin caches, skill symlinks) resolve correctly.
ARG HOST_HOME
RUN if [ -n "$HOST_HOME" ] && [ "$HOST_HOME" != "/home/developer" ]; then \
  mkdir -p "$(dirname "$HOST_HOME")" && \
  ln -snf /home/developer "$HOST_HOME"; \
  fi
RUN mkdir -p /home/developer/.config/claude/skills \
  && chown -R developer:developer /home/developer/.config

# Persist bash history.
RUN SNIPPET="export PROMPT_COMMAND='history -a' && export HISTFILE=/commandhistory/.bash_history" \
  && mkdir /commandhistory \
  && touch /commandhistory/.bash_history \
  && chown -R developer:developer /commandhistory

USER developer:developer

# Set the default editor and visual.
ENV EDITOR=nano
ENV VISUAL=nano
ENV USER=developer

ARG CLAUDE_CODE_VERSION=latest

# Install Claude Code CLI.
RUN wget -O - https://claude.ai/install.sh | bash

RUN echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.profile
