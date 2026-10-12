#!/bin/sh
# Local-only polling of the existing rendition planner/claim batch.
trap 'exit 0' TERM INT
while :; do
    media -all -limit 10 || echo 'Media batch failed; retrying in 15 seconds.' >&2
    sleep 15 &
    wait "$!"
done
