#!/usr/bin/env bash
set -euo pipefail
standalone="$(realpath "${1:?Usage: test-standalone-clean.sh STANDALONE}")"
# No GUI packages or multimedia tools are installed in this clean runtime.
# This checks the actual embedded launcher, GUI dependency resolution, and tools.
docker run --rm --network none \
  --mount "type=bind,src=$standalone,dst=/mattmux,readonly" \
  ubuntu:24.04 /mattmux --standalone-self-test
