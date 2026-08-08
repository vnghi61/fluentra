#!/bin/sh
# Initialize MinIO buckets for Fluentra
set -e

mc alias set local http://minio:9000 minioadmin minioadmin

for bucket in fluentra-avatars fluentra-media fluentra-exports; do
  mc mb --ignore-existing local/$bucket
done

# Set download policy for public assets
mc anonymous set download local/fluentra-avatars
mc anonymous set download local/fluentra-media

echo "MinIO buckets initialized successfully."
