#!/bin/bash

curl -L https://nixos.org/nix/install | sh -s -- --no-daemon
bash ~/.nix-profile/etc/profile.d/nix.sh

export PATH="/home/${DEVBOX_USER}/.nix-profile/bin:$PATH"

# Step 3: Installing devbox
export DEVBOX_USE_VERSION="0.13.7"
curl -L https://get.jetify.com/devbox | bash -s -- -f
chown -R "${DEVBOX_USER}:${DEVBOX_USER}" /usr/local/bin/devbox

KCLIPPER_URL=$(curl -s "https://api.github.com/repos/MacroPower/kclipper/releases/latest" | \
  jq -r ".assets[] | select(.name | test(\"kclipper_$(uname)_$(arch).tar.gz\")) | .browser_download_url")

echo "Downloading kclipper from $KCLIPPER_URL"
curl -L $KCLIPPER_URL | tar -zx

chmod +x kcl
mv kcl /usr/local/bin

runuser -u ubuntu renovate
