#!/usr/bin/env bash
set -euo pipefail

docker build -t ghcr.io/verboheit/quizzo:latest .
docker push ghcr.io/verboheit/quizzo:latest
docker stack deploy quizzo -c stack.yml --detach --prune --resolve-image always --with-registry-auth