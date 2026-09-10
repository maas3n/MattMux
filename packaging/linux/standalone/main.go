package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

var appVersion = "dev"

// The release script stages the built GUI and pinned multimedia tools here
// immediately before compiling this launcher. The resulting ELF is a single
// self-extracting MattMux executable.
//
//go:embed payload/*
var payloadFS embed.FS

type payloadFile struct {
	name string
	mode fs.FileMode
}

var payload = []payloadFile{
	{name: "mattmux-bin", mode: 0o755},
	{name: "ffmpeg", mode: 0o755},
	{name: "ffprobe", mode: 0o755},
	{name: "mediainfo", mode: 0o755},
}

func main() {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		fatal("this standalone build supports Linux amd64 only")
	}

	root, err := extractPayload()
	if err != nil {
		fatal(err.Error())
	}

	if len(os.Args) > 1 && os.Args[1] == "--standalone-self-test" {
		if err := selfTest(root); err != nil {
			fatal(err.Error())
		}
		fmt.Printf("MattMux %s standalone self-test: OK\n", appVersion)
		return
	}

	app := filepath.Join(root, "mattmux-bin")
	ffDir := root
	miDir := root
	path := ffDir + string(os.PathListSeparator) + miDir
	if old := os.Getenv("PATH"); old != "" {
		path += string(os.PathListSeparator) + old
	}

	env := replaceEnv(os.Environ(), "PATH", path)
	args := append([]string{app}, os.Args[1:]...)
	if err := syscall.Exec(app, args, env); err != nil {
		fatal(fmt.Sprintf("could not start MattMux: %v", err))
	}
}

func extractPayload() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("could not find user cache directory: %w", err)
	}
	if strings.TrimSpace(cache) == "" {
		return "", errors.New("user cache directory is empty")
	}

	fingerprint, err := payloadFingerprint()
	if err != nil {
		return "", err
	}
	root := filepath.Join(cache, "mattmux", "standalone", appVersion+"-"+fingerprint[:16])
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("could not create standalone cache: %w", err)
	}

	for _, p := range payload {
		data, err := payloadFS.ReadFile("payload/" + p.name)
		if err != nil {
			return "", fmt.Errorf("embedded %s is missing: %w", p.name, err)
		}
		dst := filepath.Join(root, p.name)
		if ok, err := fileMatches(dst, data, p.mode); err == nil && ok {
			continue
		}
		if err := writeAtomic(dst, data, p.mode); err != nil {
			return "", fmt.Errorf("could not extract %s: %w", p.name, err)
		}
	}
	return root, nil
}

func payloadFingerprint() (string, error) {
	h := sha256.New()
	for _, p := range payload {
		data, err := payloadFS.ReadFile("payload/" + p.name)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, p.name)
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(data)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileMatches(path string, want []byte, mode fs.FileMode) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if !st.Mode().IsRegular() || st.Size() != int64(len(want)) || st.Mode().Perm() != mode.Perm() {
		return false, nil
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.Equal(got, want), nil
}

func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".extract-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		out = append(out, item)
	}
	return append(out, prefix+value)
}

func selfTest(root string) error {
	checks := [][]string{
		{filepath.Join(root, "ffmpeg"), "-hide_banner", "-version"},
		{filepath.Join(root, "ffprobe"), "-hide_banner", "-version"},
		{filepath.Join(root, "mediainfo"), "--Version"},
	}
	for _, args := range checks {
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s self-test failed: %v: %s", filepath.Base(args[0]), err, strings.TrimSpace(string(out)))
		}
	}
	app := filepath.Join(root, "mattmux-bin")
	f, err := os.Open(app)
	if err != nil {
		return err
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return err
	}
	if !bytes.Equal(magic, []byte{0x7f, 'E', 'L', 'F'}) {
		return errors.New("embedded MattMux payload is not an ELF executable")
	}
	return nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "MattMux standalone:", msg)
	os.Exit(1)
}
