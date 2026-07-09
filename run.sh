#!/bin/sh
# Build the client if needed, then run the Cellar Floor server.
cd "$(dirname "$0")" || exit 1

if [ ! -d client/dist ]; then
  echo "client/dist missing, building client..."
  (cd client && npm install && npm run build) || exit 1
fi

exec go run ./cmd/cellarfloor "$@"
