# MattMux on Debian / Ubuntu

The Linux port provides two executables from the same source tree:

- `mattmux` — desktop GUI (Fyne)
- `mattmux-cli` — command-line interface

## Runtime tool policy

### Single-file standalone

The standalone build embeds private Wayland, X11, keyboard, and OpenGL dispatch
libraries, their dependencies, and copyright notices alongside its multimedia
tools. The launcher extracts them into its user cache and uses them through a
process-local `LD_LIBRARY_PATH`. Users do not need to install `libwayland-client0`
to start this build, and no administrator access is needed for extraction.

The host must still provide compatible glibc (the release build targets Ubuntu
24.04), a graphical session, and working graphics drivers. GPU vendor drivers
and the system ELF loader are not copied from the build runner.

`--standalone-self-test` resolves the GUI and every private shared library with
the host ELF loader before exercising the embedded command-line tools. CI also
runs this check in a clean Ubuntu container without GUI packages. Library source
package names and exact versions are recorded in the extracted
`licenses/library-packages.json`. The build downloads the exact corresponding
source packages into `MattMux-VERSION-Linux-Library-Sources.tar.gz`. Build hosts
need Debian/Ubuntu source repositories (`deb-src`) enabled; CI enables these
before building. This is a build prerequisite, not an end-user requirement.

### Other Linux packages

MattMux always checks the user's existing tools first:

1. Locate `ffmpeg` and `ffprobe` on `PATH`.
2. Verify that FFmpeg exposes the `dvdvideo` demuxer.
3. Use those system tools when the check passes.
4. If FFmpeg is missing or unsuitable, download the pinned BtbN Linux amd64 build and verify its SHA-256 before using it from the user's cache.
5. Use system `mediainfo` when available. MediaInfo is optional and is recommended by the `.deb` package.

Run `mattmux-cli tools` to inspect what MattMux sees on a machine.

## Build from source on Debian / Ubuntu

Fyne requires Go, a C compiler, and the Linux graphics development headers. Install the build prerequisites with:

```bash
sudo apt update
sudo apt install golang-go gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev git dpkg-dev
```

Then build both binaries:

```bash
cd src
go mod download
go build -o mattmux .
go build -tags cli -o mattmux-cli .
```

To create the `.deb`, portable binary tarball, source tarball, and checksums from the repository root:

```bash
bash packaging/linux/build-linux-release.sh 1.3.0-dev1
```

Install the generated `.deb` with `apt` so recommended distro tools are installed automatically when available:

```bash
sudo apt install ./dist/linux-release/mattmux_1.3.0~dev1_amd64.deb
```

MattMux does not bypass DVD copy protection such as CSS.
