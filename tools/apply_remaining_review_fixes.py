from pathlib import Path


def repl(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1))


# Publish completed Linux outputs with atomic no-replace rename where supported,
# then safe no-overwrite fallbacks for filesystems without that primitive.
Path("src/output_finalize_linux.go").write_text(r'''//go:build linux

package main

import (
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"

    "golang.org/x/sys/unix"
)

var renameOutputNoReplace = func(partial, final string) error {
    return unix.Renameat2(unix.AT_FDCWD, partial, unix.AT_FDCWD, final, unix.RENAME_NOREPLACE)
}

var linkOutputNoReplace = os.Link

func reservePartialOutput(final string) (string, error) {
    pattern := ".mattmux-" + filepath.Base(final) + ".*.partial.mkv"
    f, err := os.CreateTemp(filepath.Dir(final), pattern)
    if err != nil {
        return "", fmt.Errorf("create unique temporary output: %w", err)
    }
    name := f.Name()
    if err := f.Close(); err != nil {
        _ = os.Remove(name)
        return "", fmt.Errorf("close temporary output reservation: %w", err)
    }
    return name, nil
}

func validateAndSyncOutput(path string) error {
    st, err := os.Stat(path)
    if err != nil {
        return fmt.Errorf("FFmpeg finished but the partial MKV was not created: %w", err)
    }
    if !st.Mode().IsRegular() {
        return fmt.Errorf("FFmpeg output is not a regular file: %s", path)
    }
    if st.Size() == 0 {
        return errors.New("FFmpeg finished successfully but produced an empty MKV")
    }

    f, err := os.OpenFile(path, os.O_RDWR, 0)
    if err != nil {
        return fmt.Errorf("open completed MKV for final sync: %w", err)
    }
    if err := f.Sync(); err != nil {
        _ = f.Close()
        return fmt.Errorf("sync completed MKV: %w", err)
    }
    if err := f.Close(); err != nil {
        return fmt.Errorf("close completed MKV: %w", err)
    }
    return nil
}

func finalizeRemuxOutput(partial, final string) error {
    if err := validateAndSyncOutput(partial); err != nil {
        return err
    }
    if err := commitOutputNoReplace(partial, final); err != nil {
        return err
    }
    return syncDirectory(filepath.Dir(final))
}

func commitOutputNoReplace(partial, final string) error {
    if err := renameOutputNoReplace(partial, final); err == nil {
        return nil
    } else if errors.Is(err, unix.EEXIST) {
        return fmt.Errorf("output already exists: %s", final)
    } else if !noReplacePrimitiveUnsupported(err) {
        return fmt.Errorf("remux finished but atomic no-overwrite rename failed: %w", err)
    }

    if err := linkOutputNoReplace(partial, final); err == nil {
        _ = os.Remove(partial)
        return nil
    } else if errors.Is(err, os.ErrExist) {
        return fmt.Errorf("output already exists: %s", final)
    } else if !noReplacePrimitiveUnsupported(err) {
        return fmt.Errorf("remux finished but no-overwrite hard-link commit failed: %w", err)
    }

    // Some removable/provider-backed filesystems support neither renameat2
    // RENAME_NOREPLACE nor hard links. O_EXCL still preserves the no-overwrite
    // guarantee. If copying fails, the incomplete destination is removed and
    // the caller keeps the completed partial for recovery.
    if err := copyOutputExclusive(partial, final); err != nil {
        return fmt.Errorf("remux finished but no-overwrite copy commit failed: %w", err)
    }
    _ = os.Remove(partial)
    return nil
}

func noReplacePrimitiveUnsupported(err error) bool {
    return errors.Is(err, unix.ENOSYS) ||
        errors.Is(err, unix.EINVAL) ||
        errors.Is(err, unix.EOPNOTSUPP) ||
        errors.Is(err, unix.EPERM) ||
        errors.Is(err, unix.EXDEV)
}

func copyOutputExclusive(src, dst string) (retErr error) {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()

    out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
    if err != nil {
        if errors.Is(err, os.ErrExist) {
            return fmt.Errorf("output already exists: %s", dst)
        }
        return err
    }
    keep := false
    defer func() {
        _ = out.Close()
        if !keep {
            _ = os.Remove(dst)
        }
    }()

    if _, err := io.Copy(out, in); err != nil {
        return err
    }
    if err := out.Sync(); err != nil {
        return err
    }
    if err := out.Close(); err != nil {
        return err
    }
    keep = true
    return nil
}

func syncDirectory(dir string) error {
    f, err := os.Open(dir)
    if err != nil {
        return fmt.Errorf("open output directory for sync: %w", err)
    }
    if err := f.Sync(); err != nil {
        _ = f.Close()
        return fmt.Errorf("sync output directory: %w", err)
    }
    if err := f.Close(); err != nil {
        return fmt.Errorf("close output directory after sync: %w", err)
    }
    return nil
}
''')

# Distinguish additional titles without changing the long-standing title-1 name.
repl("src/linux_core.go", "final := outputPath(src, outDir)", "final := outputPath(src, outDir, title.Number)")
repl(
    "src/linux_core.go",
    "func outputPath(src, outDir string) string {\n",
    "func outputPath(src, outDir string, title int) string {\n",
)
repl(
    "src/linux_core.go",
    '\tif base == "" {\n\t\tbase = "DVD"\n\t}\n\treturn filepath.Join(outDir, base+".mkv")\n',
    '\tif base == "" {\n\t\tbase = "DVD"\n\t}\n\tif title > 1 {\n\t\tbase = fmt.Sprintf("%s-title-%02d", base, title)\n\t}\n\treturn filepath.Join(outDir, base+".mkv")\n',
)
repl("src/app_windows_2.go", "final := outputPath(src, outDir)", "final := outputPath(src, outDir, t.Number)")
repl(
    "src/app_windows_4.go",
    'func outputPath(src,outDir string) string { base:=filepath.Base(src);if strings.EqualFold(filepath.Ext(base),".iso"){base=strings.TrimSuffix(base,filepath.Ext(base))};if strings.EqualFold(base,"VIDEO_TS"){base=filepath.Base(filepath.Dir(src))};base=sanitizeFilename(base);if base==""{base="DVD"};return filepath.Join(outDir,base+".mkv") }',
    'func outputPath(src,outDir string,title int) string { base:=filepath.Base(src);if strings.EqualFold(filepath.Ext(base),".iso"){base=strings.TrimSuffix(base,filepath.Ext(base))};if strings.EqualFold(base,"VIDEO_TS"){base=filepath.Base(filepath.Dir(src))};base=sanitizeFilename(base);if base==""{base="DVD"};if title>1{base=fmt.Sprintf("%s-title-%02d",base,title)};return filepath.Join(outDir,base+".mkv") }',
)

# Unit coverage for unsupported rename/link publication and title-aware naming.
test = "src/release_audit_output_linux_test.go"
repl(
    test,
    '\t"time"\n)',
    '\t"time"\n\n\t"golang.org/x/sys/unix"\n)',
)
anchor = "func TestRemuxFailureCleansOwnedPartial(t *testing.T) {\n"
addition = r'''func TestCommitFallsBackWhenRenameAndLinkAreUnsupported(t *testing.T) {
    root := t.TempDir()
    partial := filepath.Join(root, ".completed.partial.mkv")
    final := filepath.Join(root, "completed.mkv")
    if err := os.WriteFile(partial, []byte("completed output"), 0600); err != nil {
        t.Fatal(err)
    }
    oldRename, oldLink := renameOutputNoReplace, linkOutputNoReplace
    renameOutputNoReplace = func(string, string) error { return unix.EOPNOTSUPP }
    linkOutputNoReplace = func(string, string) error { return unix.EOPNOTSUPP }
    t.Cleanup(func() { renameOutputNoReplace, linkOutputNoReplace = oldRename, oldLink })

    if err := commitOutputNoReplace(partial, final); err != nil {
        t.Fatalf("fallback commit failed: %v", err)
    }
    got, err := os.ReadFile(final)
    if err != nil {
        t.Fatal(err)
    }
    if string(got) != "completed output" {
        t.Fatalf("fallback output = %q", got)
    }
    if _, err := os.Stat(partial); !os.IsNotExist(err) {
        t.Fatalf("partial still exists after successful fallback: %v", err)
    }
}

func TestOutputPathDistinguishesAdditionalTitles(t *testing.T) {
    root := t.TempDir()
    src := filepath.Join(root, "disc.iso")
    if got := outputPath(src, root, 1); got != filepath.Join(root, "disc.mkv") {
        t.Fatalf("title 1 path = %q", got)
    }
    if got := outputPath(src, root, 2); got != filepath.Join(root, "disc-title-02.mkv") {
        t.Fatalf("title 2 path = %q", got)
    }
}

'''
repl(test, anchor, addition + anchor)

# Process-level SIGINT regression test for the actual CLI entry point.
Path("src/release_audit_cli_signal_linux_test.go").write_text(r'''//go:build linux && !cli

package main

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
    "time"
)

func TestCLIInterruptCleansOwnedPartial(t *testing.T) {
    root := t.TempDir()
    cli := filepath.Join(root, "mattmux-cli")
    build := exec.Command("go", "build", "-tags", "cli", "-o", cli, ".")
    if out, err := build.CombinedOutput(); err != nil {
        t.Fatalf("build CLI: %v\n%s", err, out)
    }

    bin := filepath.Join(root, "bin")
    if err := os.Mkdir(bin, 0700); err != nil {
        t.Fatal(err)
    }
    ffmpeg := `#!/bin/sh
for arg in "$@"; do
    if [ "$arg" = "-demuxers" ]; then
        printf ' D  dvdvideo\n'
        exit 0
    fi
    last="$arg"
done
printf 'partial' > "$last"
exec sleep 30
`
    if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte(ffmpeg), 0700); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(bin, "ffprobe"), []byte("#!/bin/sh\nprintf '60\\n'\n"), 0700); err != nil {
        t.Fatal(err)
    }

    src := filepath.Join(root, "disc.iso")
    if err := os.WriteFile(src, []byte("fixture"), 0600); err != nil {
        t.Fatal(err)
    }
    outDir := filepath.Join(root, "out")
    if err := os.Mkdir(outDir, 0700); err != nil {
        t.Fatal(err)
    }

    cmd := exec.Command(cli, "remux", "--title", "1", "--output", outDir, src)
    cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
    if err := cmd.Start(); err != nil {
        t.Fatal(err)
    }

    deadline := time.Now().Add(8 * time.Second)
    found := false
    for time.Now().Before(deadline) {
        matches, _ := filepath.Glob(filepath.Join(outDir, ".mattmux-*.partial.mkv"))
        if len(matches) > 0 {
            found = true
            break
        }
        time.Sleep(25 * time.Millisecond)
    }
    if !found {
        _ = cmd.Process.Kill()
        _ = cmd.Wait()
        t.Fatal("CLI did not create a partial output before interrupt")
    }

    if err := cmd.Process.Signal(os.Interrupt); err != nil {
        _ = cmd.Process.Kill()
        _ = cmd.Wait()
        t.Fatal(err)
    }
    if err := cmd.Wait(); err == nil {
        t.Fatal("interrupted CLI unexpectedly exited successfully")
    }
    entries, err := os.ReadDir(outDir)
    if err != nil {
        t.Fatal(err)
    }
    for _, entry := range entries {
        if filepath.Ext(entry.Name()) == ".mkv" || filepath.Base(entry.Name()) != "" && len(entry.Name()) > 0 && filepath.Clean(entry.Name()) == entry.Name() {
            if matched, _ := filepath.Match(".mattmux-*.partial.mkv", entry.Name()); matched {
                t.Fatalf("Ctrl+C left partial output %s", entry.Name())
            }
        }
    }
}
''')

# Documentation and maintenance cleanup from the review.
repl(
    ".gitignore",
    "# OS / editor files\n",
    "# Python generated files\n__pycache__/\n*.pyc\n\n# OS / editor files\n",
)
pyc = Path("tools/__pycache__/apply-track-selection.cpython-312.pyc")
if pyc.exists():
    pyc.unlink()

repl(
    "android/README.md",
    "This directory contains the experimental Android/ChromeOS frontend for MattMux. The current public milestone is **1.2.0 Alpha 4** (`v1.2.0-chromeos-alpha4`).\n\n## Alpha 4 status\n",
    "This directory contains the experimental Android/ChromeOS frontend for MattMux. Android/ChromeOS follows the unified MattMux release line; the latest published unified release at the time of writing is **1.4.0** (`v1.4.0`).\n\n## Experimental status\n",
)
repl(
    "android/README.md",
    "- production Play purchase verification/signing/rollout remains separate from the Alpha 4 APK release\n",
    "- production Play purchase verification and rollout remain separate from the experimental GitHub APK\n- v1.4.0 was debug-signed; a later persistently signed APK must not be assumed to update that installation in place\n",
)
repl(
    "android/README.md",
    "but purchases are deliberately disabled in the current Alpha 4 build through `BuildConfig.ENABLE_BILLING_PURCHASES = false`.",
    "but purchases are deliberately disabled in the current experimental build through `BuildConfig.ENABLE_BILLING_PURCHASES = false`.",
)

repl(
    "RELEASING.md",
    "6. Keep historical platform-specific releases available for provenance; the unified model applies to new releases.\n",
    "6. Keep historical development provenance in Git history; obsolete platform-specific public release entries may remain retired after the unified-release cleanup.\n",
)
repl(
    "RELEASING.md",
    "The existing Windows `v1.2.0`, Linux `v1.3.0-dev5`, and ChromeOS alpha tags/releases are historical releases from before the unified release model. Leave those tags and assets intact. New releases use the unified version namespace only.\n",
    "Windows `v1.2.0`, Linux `v1.3.0-dev5`, and the old ChromeOS alpha line are historical development lines from before the unified release model. Their development remains in Git history; obsolete public release entries/tags were retired during the unified-release cleanup. Do not recreate platform-specific release lines. New releases use the unified version namespace only.\n",
)
repl(
    "RELEASING.md",
    "Android/ChromeOS purchases remain disabled until production device validation, signing, and purchase-verification readiness are complete. The separate Play bundle workflow is distribution tooling; it does not create GitHub Releases.\n",
    "Android/ChromeOS purchases remain disabled until production device validation, signing, and purchase-verification readiness are complete. The separate Play bundle workflow is distribution tooling; it does not create GitHub Releases.\n\nGitHub release APKs after v1.4.0 use the persistent Android upload-signing credentials. The v1.4.0 APK was debug-signed, so an in-place upgrade from that APK must not be promised unless its original debug key is proven compatible. Release notes for the first persistently signed APK must call out the migration requirement. Preserve the same distribution key for subsequent APK releases and verify its certificate before publishing.\n",
)

readme_anchor = "### Android / ChromeOS\n\nDownload `MattMux-1.4.0-ChromeOS.apk` from the [MattMux 1.4.0 release](https://github.com/maas3n/MattMux/releases/tag/v1.4.0) and install it on a compatible Android/ChromeOS device.\n\n"
readme_extra = readme_anchor + "### Output location behavior\n\nThe Windows and Linux desktop GUIs default to the user's Videos directory (or home) and remember the chosen output folder. `mattmux-cli remux` instead writes to the current working directory when `--output` is omitted. The Windows All-in-One launcher may use its extraction directory as the child working directory, so the GUI output field remains authoritative.\n\n"
repl("README.md", readme_anchor, readme_extra)

repl(
    "CHANGELOG.md",
    "# Changelog\n\n",
    "# Changelog\n\n## Unreleased\n\n- Preserve completed desktop and Android remuxes when final publication fails, and add Linux no-overwrite publication fallbacks for filesystems without hard links.\n- Use consistent 100M probe/analyze limits for desktop metadata probing and remuxing.\n- Clean Linux CLI partial outputs on Ctrl+C/SIGTERM and add a process-level interrupt regression test.\n- Correct Debian preview ordering and declare the measured `libc6 (>= 2.38)` runtime floor.\n- Give additional DVD titles distinct `-title-NN` desktop output filenames.\n- Sign future GitHub Android APKs with persistent distribution credentials and reject debug certificates.\n- Carry DVD IFO language mappings and the selected PGC subtitle palette into Android native stream metadata.\n- Align Android/release documentation and remove tracked Python bytecode.\n\n",
)
