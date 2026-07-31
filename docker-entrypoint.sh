#!/bin/sh
set -e

# A bind-mounted ./data arrives owned by the host user (often root), which
# the unprivileged runtime user cannot write to. Fix ownership while we
# still have the privileges to do so, then drop them for good.
#
# When the container is already running as a non-root user (docker run
# --user, compose `user:`), there is nothing to fix and nothing to drop.
if [ "$(id -u)" = "0" ]; then
    DATA_DIR="${FEEDFORGE_DATA:-/data}"
    mkdir -p "$DATA_DIR"
    if [ "$(stat -c %u "$DATA_DIR")" != "$(id -u feedforge)" ]; then
        chown -R feedforge:feedforge "$DATA_DIR" 2>/dev/null || \
            echo "warning: could not chown $DATA_DIR; continuing" >&2
    fi
    exec su-exec feedforge:feedforge feedforge "$@"
fi

exec feedforge "$@"
