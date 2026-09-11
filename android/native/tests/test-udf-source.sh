#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
UDF_SOURCE="${1:?Pass the pinned libudfread source directory}"
TEST_WORK="$(mktemp -d)"
trap 'rm -rf "$TEST_WORK"' EXIT
python3 "${ROOT}/android/native/tests/make_udf_fixture.py" "${TEST_WORK}"
cc -std=c11 -D_POSIX_C_SOURCE=200809L -DHAVE_UNISTD_H=1 -DHAVE_FCNTL_H=1 \
  -Wall -Wextra -fsanitize=address,undefined -g \
  -I"${UDF_SOURCE}/src" -I"${ROOT}/android/native" \
  "${UDF_SOURCE}/src/udfread.c" "${UDF_SOURCE}/src/ecma167.c" \
  "${UDF_SOURCE}/src/default_blockinput.c" \
  "${ROOT}/android/native/udf_source.c" "${ROOT}/android/native/tests/udf_source_test.c" \
  -o "${TEST_WORK}/udf-source-test"
"${TEST_WORK}/udf-source-test" "${TEST_WORK}/fixture.iso" "${TEST_WORK}/VIDEO_TS"
