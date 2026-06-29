#!/bin/bash
set -euo pipefail

DOCKERFILE="tools/kessel-debug-container/Dockerfile"

LIBRDKAFKA_MIRROR="https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/"
KCAT_MIRROR="https://dl.fedoraproject.org/pub/epel/9/Everything/x86_64/Packages/k/"

echo "Checking for latest librdkafka x86_64 RPM..."
LIBRDKAFKA_LATEST=$(curl -fsSL "$LIBRDKAFKA_MIRROR" \
  | grep -oP 'href="librdkafka-\K[^"]+(?=\.el9\.x86_64\.rpm")' \
  | sort -V \
  | tail -n 1)

if [ -z "$LIBRDKAFKA_LATEST" ]; then
  echo "ERROR: Could not determine latest librdkafka version" >&2
  exit 1
fi

echo "Checking for latest kcat x86_64 RPM..."
KCAT_LATEST=$(curl -fsSL "$KCAT_MIRROR" \
  | grep -oP 'href="kcat-\K[^"]+(?=\.el9\.x86_64\.rpm")' \
  | sort -V \
  | tail -n 1)

if [ -z "$KCAT_LATEST" ]; then
  echo "ERROR: Could not determine latest kcat version" >&2
  exit 1
fi

LIBRDKAFKA_CURRENT=$(grep -oP 'LIBRDKAFKA_VERSION=\K.+' "$DOCKERFILE")
KCAT_CURRENT=$(grep -oP 'KCAT_VERSION=\K.+' "$DOCKERFILE")

echo ""
echo "librdkafka: current=${LIBRDKAFKA_CURRENT} latest=${LIBRDKAFKA_LATEST}"
echo "kcat:       current=${KCAT_CURRENT} latest=${KCAT_LATEST}"

UPDATED=0

if [ "$LIBRDKAFKA_CURRENT" != "$LIBRDKAFKA_LATEST" ]; then
  sed -i "s/LIBRDKAFKA_VERSION=${LIBRDKAFKA_CURRENT}/LIBRDKAFKA_VERSION=${LIBRDKAFKA_LATEST}/" "$DOCKERFILE"
  echo "Updated librdkafka: ${LIBRDKAFKA_CURRENT} -> ${LIBRDKAFKA_LATEST}"
  UPDATED=1
fi

if [ "$KCAT_CURRENT" != "$KCAT_LATEST" ]; then
  sed -i "s/KCAT_VERSION=${KCAT_CURRENT}/KCAT_VERSION=${KCAT_LATEST}/" "$DOCKERFILE"
  echo "Updated kcat: ${KCAT_CURRENT} -> ${KCAT_LATEST}"
  UPDATED=1
fi

if [ "$UPDATED" -eq 0 ]; then
  echo "Already up to date."
fi
