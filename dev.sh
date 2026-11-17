#!/usr/bin/env bash
# Development: backend (:8787), recording proxy (:8788), and Next.js frontend (:3000).
# Frontend /api requests proxy to the backend; Ctrl-C stops both processes.
#
# For an embedded single binary, follow the README build instructions instead.
set -euo pipefail
cd "$(dirname "$0")"

# Stop all child processes in this process group on exit.
cleanup() { kill 0 2>/dev/null || true; }
trap cleanup EXIT INT TERM

# Backend without embedded frontend; configure worker concurrency in System Settings.
go run ./cmd/antifox -addr :8787 -proxy :8788 &

# Frontend hot reload with /api proxied to :8787.
( cd web && npm run dev ) &

echo "[dev] Backend :8787 / proxy :8788 / frontend http://localhost:3000  (Ctrl-C to exit)"
wait
