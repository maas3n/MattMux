#!/usr/bin/env python3
"""Stage private GUI/tool dependencies from the Debian/Ubuntu build host."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

# Keep glibc and the loader paired with the host. GPU vendor drivers also
# remain host-provided; copying a build runner's drivers would break users' GPUs.
HOST_LIBS = {
    "libc.so.6", "libm.so.6", "libpthread.so.0", "libdl.so.2",
    "librt.so.1", "libresolv.so.2", "libutil.so.1", "ld-linux-x86-64.so.2",
}
# GLFW also opens these with dlopen, so ldd alone is insufficient.
GUI_LIBS = (
    "libwayland-client.so.0", "libwayland-cursor.so.0", "libwayland-egl.so.1",
    "libxkbcommon.so.0", "libX11.so.6", "libXcursor.so.1", "libXrandr.so.2",
    "libXinerama.so.1", "libXi.so.6", "libXxf86vm.so.1",
    "libGL.so.1", "libEGL.so.1", "libGLX.so.0",
)


def run(*args):
    return subprocess.check_output(args, text=True, env={**os.environ, "LC_ALL": "C"})


def dependencies(path):
    result = subprocess.run(["ldd", str(path)], text=True, capture_output=True,
                            env={**os.environ, "LC_ALL": "C"})
    output = result.stdout + result.stderr
    if "not found" in output:
        raise RuntimeError(f"Unresolved build dependency for {path}:\n{output}")
    if result.returncode:
        if "statically linked" in output or "not a dynamic executable" in output:
            return {}
        raise RuntimeError(f"Cannot inspect {path}:\n{output}")
    return dict(re.findall(r"^\s*(\S+) => (/\S+) \(", output, re.MULTILINE))


def package_info(path):
    # Account for both spellings on merged-/usr Debian/Ubuntu systems.
    candidates = {str(path), str(Path(path).resolve())}
    for candidate in list(candidates):
        if candidate.startswith("/usr/lib/"):
            candidates.add(candidate[4:])
    for candidate in sorted(candidates):
        result = subprocess.run(["dpkg-query", "-S", candidate], text=True, capture_output=True)
        if result.returncode == 0:
            package = result.stdout.split(": ", 1)[0]
            fields = run("dpkg-query", "-W", "-f=${Package}\t${Version}\t${source:Package}\t${source:Version}", package).split("\t")
            return dict(zip(("package", "version", "source", "source_version"), fields))
    raise RuntimeError(f"No Debian package provenance for {path}")


def bundle(payload):
    payload = Path(payload)
    libs = payload / "lib"
    notices = payload / "licenses"
    libs.mkdir()
    notices.mkdir()
    cache = {}
    for name, path in re.findall(r"^\s*(\S+) \(libc6,x86-64[^)]*\) => (/\S+)", run("ldconfig", "-p"), re.MULTILINE):
        cache.setdefault(name, path)
    pending = [p for p in payload.iterdir() if p.is_file() and os.access(p, os.X_OK)]
    required = {}
    for name in GUI_LIBS:
        if name not in cache:
            raise RuntimeError(f"Build host is missing required GUI library {name}")
        required[name] = cache[name]
    pending.extend(Path(p) for p in required.values())
    for path in pending:
        required.update(dependencies(path))  # ldd resolves the transitive closure.
    manifest = {}
    for name, path in sorted(required.items()):
        if name in HOST_LIBS:
            continue
        info = package_info(path)
        shutil.copyfile(path, libs / name)
        (libs / name).chmod(0o644)
        manifest[name] = info
        copyright_file = Path("/usr/share/doc") / info["package"] / "copyright"
        if not copyright_file.is_file():
            raise RuntimeError(f"Missing copyright notice: {copyright_file}")
        shutil.copyfile(copyright_file, notices / (info["package"] + "-copyright"))
    shutil.copytree("/usr/share/common-licenses", notices / "common-licenses", symlinks=False)
    (notices / "library-packages.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print(f"Bundled {len(manifest)} shared libraries in {libs}")


if __name__ == "__main__":
    bundle(sys.argv[1])
