#!/usr/bin/env python3
"""Download exact Debian source packages corresponding to private libraries."""
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile

manifest, destination = map(Path, sys.argv[1:])
packages = json.loads(manifest.read_text())
stage = destination.parent / "standalone-library-sources"
stage.mkdir()
shutil.copyfile(manifest, stage / manifest.name)
sources = sorted({(p["source"], p["source_version"]) for p in packages.values()})
for name, version in sources:
    target = stage / name
    target.mkdir()
    # APT authenticates the source index and verifies the downloads against it.
    # Never silently substitute a newer source version for the binary in use.
    subprocess.run(["apt-get", "source", "--download-only", "--only-source",
                    f"{name}={version}"], cwd=target, check=True)
    if not list(target.glob("*.dsc")):
        raise RuntimeError(f"No source descriptor downloaded for {name}={version}")
with tarfile.open(destination, "w:gz") as archive:
    archive.add(stage, arcname="standalone-library-sources")
print(f"Archived {len(sources)} exact library source packages: {destination}")
