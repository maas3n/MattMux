#!/usr/bin/env python3
"""Stage private GUI/tool dependencies from the Debian/Ubuntu build host."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

# Keep glibc and the loader paired with the host. Hardware drivers remain
# host-provided. A separate Mesa software stack is used only as a fallback.
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
    # GLVND loads vendors and Mesa loads DRI drivers with dlopen, outside ldd.
    # Keep this stack isolated so it cannot replace a working host GPU driver.
    software = {name: cache[name] for name in ("libGLX_mesa.so.0", "libEGL_mesa.so.0")}
    dri = Path("/usr/lib/x86_64-linux-gnu/dri/swrast_dri.so")
    if not dri.is_file():
        raise RuntimeError("Build host needs libgl1-mesa-dri for software rendering")
    software["dri/swrast_dri.so"] = str(dri)
    for path in list(software.values()):
        software.update(dependencies(path))
    # Newer Mesa shares the implementation through a separately loaded object.
    for path in dri.parent.glob("libgallium*.so*"):
        software[path.name] = str(path)
        software.update(dependencies(path))
    files = {"lib/" + name: path for name, path in required.items()}
    files.update({"software/" + name: path for name, path in software.items()})
    manifest = {}
    for relative, path in sorted(files.items()):
        name = Path(relative).name
        if name in HOST_LIBS:
            continue
        info = package_info(path)
        target = payload / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, target)
        target.chmod(0o644)
        manifest[relative] = info
        copyright_file = Path("/usr/share/doc") / info["package"] / "copyright"
        if not copyright_file.is_file():
            raise RuntimeError(f"Missing copyright notice: {copyright_file}")
        shutil.copyfile(copyright_file, notices / (info["package"] + "-copyright"))
    shutil.copytree("/usr/share/common-licenses", notices / "common-licenses", symlinks=False)
    (notices / "library-packages.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print(f"Bundled {len(manifest)} shared libraries in {libs}")


if __name__ == "__main__":
    bundle(sys.argv[1])
