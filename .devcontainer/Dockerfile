FROM jetpackio/devbox:latest

ARG TZ
ENV TZ="$TZ"

# Installing devbox project
WORKDIR /workspace
USER root:root
RUN mkdir -p /workspace && chown ${DEVBOX_USER}:${DEVBOX_USER} /workspace

# Install basic development tools and iptables/ipset
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
  && apt-get clean && rm -rf /var/lib/apt/lists/*

# Map host home path into the container so mounted configs with absolute
# host paths (e.g. plugin caches, skill symlinks) resolve correctly.
ARG HOST_HOME
RUN if [ -n "$HOST_HOME" ] && [ "$HOST_HOME" != "/home/devbox" ]; then \
  mkdir -p "$(dirname "$HOST_HOME")" && \
  ln -snf /home/devbox "$HOST_HOME"; \
  fi
RUN mkdir -p /home/devbox/.config/claude/skills \
  && chown -R ${DEVBOX_USER}:${DEVBOX_USER} /home/devbox/.config

# Persist bash history.
RUN SNIPPET="export PROMPT_COMMAND='history -a' && export HISTFILE=/commandhistory/.bash_history" \
  && mkdir /commandhistory \
  && touch /commandhistory/.bash_history \
  && chown -R ${DEVBOX_USER}:${DEVBOX_USER} /commandhistory

USER ${DEVBOX_USER}:${DEVBOX_USER}
COPY --chown=${DEVBOX_USER}:${DEVBOX_USER} devbox.json devbox.json
COPY --chown=${DEVBOX_USER}:${DEVBOX_USER} devbox.lock devbox.lock

RUN devbox run -- echo "Installed Packages." && nix-store --gc && nix-store --optimise

# Set the default editor and visual
ENV EDITOR=nano
ENV VISUAL=nano
ENV USER=${DEVBOX_USER}

ARG CLAUDE_CODE_VERSION=latest

# Install Claude Code CLI
RUN devbox run -- wget -O - https://claude.ai/install.sh | bash

RUN echo 'eval "$(devbox shellenv --init-hook)"' >> ~/.profile
RUN echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.profile
