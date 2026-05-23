#!/usr/bin/env bash
set -euo pipefail

git pull
docker build -t ghcr.io/verboheit/quizzo:latest .
docker pull ghcr.io/verboheit/quizzo:latest
docker stack rm quizzo
docker stack deploy quizzo -c stack.yml --detach --prune --resolve-image always --with-registry-auth