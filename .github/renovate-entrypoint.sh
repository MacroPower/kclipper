#!/bin/bash

KCLIPPER_URL=$(curl -s "https://api.github.com/repos/macropower/kclipper/releases/latest" | \
  jq -r ".assets[] | select(.name | test(\"kclipper_$(uname)_$(arch).tar.gz\")) | .browser_download_url")

echo "Downloading kclipper from $KCLIPPER_URL"
curl -L $KCLIPPER_URL | tar -zx

chmod +x kcl
mv kcl /usr/local/bin

runuser -u ubuntu renovate
