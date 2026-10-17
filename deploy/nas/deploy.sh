#!/bin/bash
# Envoie le code sur le NAS et relance la stack (image construite sur place).
# Usage : deploy/nas/deploy.sh   (depuis la racine du dépôt ; NASA=hôte ssh, par défaut nasa)
set -euo pipefail
HOST=${NASA:-nasa}
PROJ=/boot/config/plugins/compose.manager/projects/orapa
rsync -az --delete --exclude .git --exclude node_modules --exclude 'web/dist' --exclude data --exclude '*.db' ./ "$HOST:/mnt/user/appdata/orapa/src/"
ssh "$HOST" "mkdir -p /mnt/user/appdata/orapa/data && chown 65532:65532 /mnt/user/appdata/orapa/data \
  && cp /mnt/user/appdata/orapa/src/deploy/nas/compose.yaml /mnt/user/appdata/orapa/src/deploy/nas/compose.override.yaml $PROJ/ \
  && cd $PROJ && docker compose -f compose.yaml -f compose.override.yaml up -d --build"
