#!/bin/sh
set -eu

mkdir -p /data
chown -R app:app /data

if [ "$#" -eq 0 ]; then
  set -- /app/id
elif [ "${1#-}" != "$1" ]; then
  set -- /app/id "$@"
fi

exec gosu app "$@"
