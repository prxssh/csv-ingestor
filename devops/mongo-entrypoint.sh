#!/bin/bash
set -e

mkdir -p /etc/mongo
cp /tmp/mongo-keyfile-src /etc/mongo/mongo-keyfile
chown 999:999 /etc/mongo/mongo-keyfile
chmod 400 /etc/mongo/mongo-keyfile

exec /usr/local/bin/docker-entrypoint.sh "$@"
