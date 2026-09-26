#!/usr/bin/env bash
set -euo pipefail
standalone="$(realpath "${1:?Usage: test-standalone-clean.sh STANDALONE}")"
# No GUI packages or multimedia tools are installed in this clean runtime.
# This checks the actual embedded launcher, GUI dependency resolution, and tools.
docker run --rm --network none \
  --mount "type=bind,src=$standalone,dst=/mattmux,readonly" \
  ubuntu:24.04 /mattmux --standalone-self-test

# The X server lives outside the container. Its client still has no host Mesa,
# Wayland, X11 or multimedia packages and must use the embedded renderer.
work="$(mktemp -d)"
Xvfb -displayfd 3 -screen 0 1024x768x24 -ac -nolisten tcp 3>"$work/display" >"$work/xvfb.log" 2>&1 &
xvfb_pid=$!
trap 'kill "$xvfb_pid" 2>/dev/null || true; rm -rf "$work"' EXIT
for attempt in $(seq 1 100); do
  [[ -s "$work/display" ]] && break
  kill -0 "$xvfb_pid" || { cat "$work/xvfb.log"; exit 1; }
  sleep 0.1
done
test -s "$work/display"
export DISPLAY=":$(cat "$work/display")"

docker run --rm --network none \
  --env DISPLAY \
  --mount type=bind,src=/tmp/.X11-unix,dst=/tmp/.X11-unix,readonly \
  --mount "type=bind,src=$standalone,dst=/mattmux,readonly" \
  ubuntu:24.04 bash -euc '
    timeout 120 /mattmux --graphics-self-test > /tmp/graphics.log 2>&1
    cat /tmp/graphics.log
    grep -F "using bundled software rendering" /tmp/graphics.log
    grep -F "MattMux GUI graphics self-test: OK" /tmp/graphics.log
    root=$(echo /root/.cache/mattmux/standalone/*)
    # Prove success above depended on the bundled DRI driver, not a host one.
    rm "$root/software/dri/swrast_dri.so"
    if LD_LIBRARY_PATH="$root/software:$root/lib" \
      LIBGL_DRIVERS_PATH="$root/software/dri" LIBGL_ALWAYS_SOFTWARE=1 \
      GALLIUM_DRIVER=llvmpipe __GLX_VENDOR_LIBRARY_NAME=mesa \
      timeout 15 "$root/graphics-probe"; then
      echo "Unexpected graphics success without bundled renderer" >&2
      exit 1
    fi
  '

# A working host Mesa driver must still be preferred over the fallback.
XDG_CACHE_HOME="$work/cache" timeout 120 "$standalone" --graphics-self-test >"$work/host.log" 2>&1
cat "$work/host.log"
grep -F 'MattMux GUI graphics self-test: OK' "$work/host.log"
if grep -F 'using bundled software rendering' "$work/host.log"; then
  echo 'Working host driver was not used' >&2
  exit 1
fi
