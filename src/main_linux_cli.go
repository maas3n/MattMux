//go:build linux && cli

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	if len(os.Args) == 1 {
		printCLIUsage()
		return
	}
	if os.Args[1] == "--version" || os.Args[1] == "-version" || os.Args[1] == "version" {
		fmt.Printf("MattMux CLI %s\n", appVersion)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if os.Args[1] == "--batch" {
		cliBatch(ctx, os.Args[2:])
		return
	}
	switch os.Args[1] {
	case "tools", "doctor":
		fmt.Println(toolSummary(ctx))
	case "scan":
		cliScan(ctx, os.Args[2:])
	case "metadata":
		cliMetadata(ctx, os.Args[2:])
	case "remux":
		cliRemux(ctx, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printCLIUsage()
		os.Exit(2)
	}
}

func printCLIUsage() {
	fmt.Printf(`MattMux CLI %s

Usage:
  mattmux-cli tools
  mattmux-cli scan SOURCE
  mattmux-cli metadata [--title N] SOURCE
  mattmux-cli remux [--title N] [--output DIR] [--no-chapters] SOURCE
  mattmux-cli --batch [--log FILE] MOVIES_ROOT [OUTPUT_ROOT]
  mattmux-cli --version

SOURCE may be a DVD directory/VIDEO_TS structure or an ISO image.
If --title is omitted, MattMux scans the disc and selects the longest title.

Batch mode expects MOVIES_ROOT to contain movie folders with VIDEO_TS subfolders.
Each movie is scanned through FFmpeg dvdvideo/libdvdread/libdvdnav, the longest title
is selected automatically, and all streams are remuxed losslessly to MKV. When
OUTPUT_ROOT is omitted, each MKV is written into its corresponding movie folder.
--log is optional and appends batch activity to the chosen file.

The CLI first uses compatible ffmpeg/ffprobe binaries already installed on PATH.
If system FFmpeg lacks the dvdvideo demuxer, MattMux prepares its pinned fallback.
`, appVersion)
}

func cliStatus(frac float64, status string) {
	if status == "" {
		return
	}
	if frac > 0 && frac < 1 {
		fmt.Fprintf(os.Stderr, "[%3d%%] %s\n", int(frac*100), status)
	} else {
		fmt.Fprintln(os.Stderr, status)
	}
}

func cliBatch(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("--batch", flag.ExitOnError)
	logPath := fs.String("log", "", "optional batch log file")
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "Usage: mattmux-cli --batch [--log FILE] MOVIES_ROOT [OUTPUT_ROOT]") }
	_ = fs.Parse(args)
	if fs.NArg() < 1 || fs.NArg() > 2 {
		fs.Usage()
		os.Exit(2)
	}
	inputRoot := fs.Arg(0)
	outputRoot := ""
	if fs.NArg() == 2 {
		outputRoot = fs.Arg(1)
	}
	var logFile *os.File
	if strings.TrimSpace(*logPath) != "" {
		abs, err := filepath.Abs(strings.TrimSpace(*logPath))
		fatalIf(err)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			fatalIf(fmt.Errorf("create log folder: %w", err))
		}
		logFile, err = os.OpenFile(abs, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		fatalIf(err)
		defer logFile.Close()
		fmt.Fprintf(os.Stderr, "Batch log: %s\n", abs)
	}
	result, err := runBatch(ctx, batchOptions{InputRoot: inputRoot, OutputRoot: outputRoot, Log: logFile}, func(frac float64, status string) {
		cliStatus(frac, status)
	})
	for _, output := range result.Outputs {
		fmt.Println(output)
	}
	fatalIf(err)
}

func cliScan(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "Usage: mattmux-cli scan SOURCE") }
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	src := fs.Arg(0)
	tools := mustTools(ctx, false)
	titles, err := scanTitles(ctx, src, tools, cliStatus)
	fatalIf(err)
	best, _ := longestTitle(titles)
	fmt.Printf("%-7s %-12s %s\n", "TITLE", "DURATION", "DEFAULT")
	for _, t := range titles {
		mark := ""
		if t.Number == best.Number {
			mark = "longest"
		}
		fmt.Printf("%-7d %-12s %s\n", t.Number, formatDuration(t.Duration), mark)
	}
}
func cliMetadata(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("metadata", flag.ExitOnError)
	titleN := fs.Int("title", 0, "DVD title number; 0 selects the longest title")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: mattmux-cli metadata [--title N] SOURCE")
		os.Exit(2)
	}
	src := fs.Arg(0)
	tools := mustTools(ctx, true)
	title := mustResolveTitle(ctx, src, *titleN, tools)
	text, err := metadataText(ctx, src, title, tools, true)
	fatalIf(err)
	fmt.Print(text)
}
func cliRemux(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("remux", flag.ExitOnError)
	titleN := fs.Int("title", 0, "DVD title number; 0 selects the longest title")
	output := fs.String("output", "", "output directory (default: current directory)")
	noChapters := fs.Bool("no-chapters", false, "do not preserve DVD chapter markers")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: mattmux-cli remux [--title N] [--output DIR] [--no-chapters] SOURCE")
		os.Exit(2)
	}
	src := fs.Arg(0)
	outDir := strings.TrimSpace(*output)
	if outDir == "" {
		var err error
		outDir, err = os.Getwd()
		fatalIf(err)
	}
	outDir, _ = filepath.Abs(outDir)
	tools := mustTools(ctx, false)
	title := mustResolveTitle(ctx, src, *titleN, tools)
	fmt.Fprintf(os.Stderr, "Using %s FFmpeg: %s\n", tools.source, tools.ffmpeg)
	final, err := remuxTitle(ctx, src, title, outDir, !*noChapters, nil, tools, cliStatus)
	fatalIf(err)
	fmt.Println(final)
}
func mustTools(ctx context.Context, needMediaInfo bool) toolPaths {
	tools, err := ensureTools(ctx, needMediaInfo, cliStatus)
	fatalIf(err)
	return tools
}
func mustResolveTitle(ctx context.Context, src string, requested int, tools toolPaths) titleInfo {
	if requested < 0 {
		fatalIf(fmt.Errorf("title must be >= 0"))
	}
	if requested > 0 {
		d, err := probeDuration(ctx, tools.ffprobe, src, requested)
		fatalIf(err)
		return titleInfo{Number: requested, Duration: d}
	}
	titles, err := scanTitles(ctx, src, tools, cliStatus)
	fatalIf(err)
	best, err := longestTitle(titles)
	fatalIf(err)
	fmt.Fprintf(os.Stderr, "Selected longest title %d (%s).\n", best.Number, formatDuration(best.Duration))
	return best
}
func fatalIf(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "mattmux-cli:", err)
	os.Exit(1)
}
func parseTitleLabel(label string) int {
	fields := strings.Fields(label)
	if len(fields) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(fields[1])
	return n
}
