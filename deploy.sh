#!/usr/bin/env bash
set -euo pipefail

git pull
docker stack rm quizzo
docker build -t quizzo:latest .
docker stack deploy quizzo -c stack.yml --detach --prune --resolve-image always --with-registry-auth