package io.github.maas3n.mattmux;

import java.nio.file.*;
import java.util.*;

/** Exercise the real scan -> plan -> remux path used by all Android DVD tabs. */
public final class DvdTitleRegression {
    private static void require(boolean condition, String message) {
        if (!condition) throw new AssertionError(message);
    }

    public static void main(String[] args) throws Exception {
        System.load(args[0]);
        Path work = Path.of(args[1]);
        AndroidNativeRemuxEngine engine = new AndroidNativeRemuxEngine();
        for (String name : new String[]{"long-first", "long-last", "single"}) {
            Path disc = work.resolve(name);
            int expectedTitle = name.equals("long-last") ? 2 : 1;
            int count = name.equals("single") ? 1 : 2;
            int isoFd = AndroidNativeRemuxEngine.openPath(work.resolve(name + ".iso").toString(), false);
            long iso = engine.nativeOpenIso(isoFd);
            AndroidNativeRemuxEngine.closePath(isoFd);
            require(iso != 0, "Could not open " + name);
            try {
                for (boolean fromIso : new boolean[]{false, true}) {
                    Path stage = work.resolve(name + (fromIso ? "-iso-stage" : "-folder-stage"));
                    Files.createDirectories(stage.resolve("VIDEO_TS"));
                    if (fromIso) {
                        Files.write(stage.resolve("VIDEO_TS/VIDEO_TS.IFO"), engine.nativeReadIsoIfo(iso, 0));
                        for (int ts = 1; ts <= 99; ts++) {
                            byte[] bytes = engine.nativeReadIsoIfo(iso, ts);
                            if (bytes != null) Files.write(stage.resolve(String.format(Locale.ROOT, "VIDEO_TS/VTS_%02d_0.IFO", ts)), bytes);
                        }
                    } else {
                        try (var files = Files.newDirectoryStream(disc.resolve("VIDEO_TS"), "*.IFO")) {
                            for (Path file : files) Files.copy(file, stage.resolve("VIDEO_TS").resolve(file.getFileName()), StandardCopyOption.REPLACE_EXISTING);
                        }
                    }
                    long[] scan = engine.nativeScanDvdNav(stage.toString());
                    require(scan != null && scan[0] == count, "Missing title scan for " + name);
                    require(scan[1] == expectedTitle, "Wrong longest title for " + name + ": " + Arrays.toString(scan));
                    for (int title = 1; title <= count; title++) {
                        long expectedMs = title == expectedTitle ? 6000 : 2000;
                        require(Math.abs(scan[title + 2] / 90 - expectedMs) < 150, "Wrong duration for title " + title);
                        String[] plan = engine.nativePlanDvdNav(stage.toString(), title);
                        require(plan != null, "No plan for " + name + " title " + title);
                        String[] header = plan[0].split("\t");
                        require(header[0].equals("T") && Integer.parseInt(header[1]) == title, "Wrong planned title");
                        require(Math.abs(Long.parseLong(header[3]) - expectedMs) < 150, "Plan describes another title's duration");
                        if (title == expectedTitle) remux(engine, disc, work.resolve(name + (fromIso ? "-iso.mkv" : "-folder.mkv")), plan, fromIso ? iso : 0);
                    }
                    require(engine.nativePlanDvdNav(stage.toString(), 0) == null, "Accepted title zero");
                    require(engine.nativePlanDvdNav(stage.toString(), count + 1) == null, "Accepted nonexistent title");
                }
            } finally { engine.nativeCloseIso(iso); }
        }
        demuxAuthoredDvd(engine, work, false);
        demuxAuthoredDvd(engine, work, true);
        System.out.println("Production JNI longest/explicit title selection, staged folder/ISO planning and remux PASS");
    }

    private static void demuxAuthoredDvd(AndroidNativeRemuxEngine engine, Path work, boolean reset) throws Exception {
        Path disc = work.resolve(reset ? "demux-dvd/clock-reset" : "demux-dvd/dvd");
        String[] plan = engine.nativePlanDvdNav(disc.toString(), 1);
        require(plan != null, "Could not plan subtitled DVD");
        int isoFd = AndroidNativeRemuxEngine.openPath(work.resolve(reset ? "clock-reset.iso" : "demux-dvd.iso").toString(), false);
        long iso = engine.nativeOpenIso(isoFd);
        AndroidNativeRemuxEngine.closePath(isoFd);
        require(iso != 0, "Could not open subtitled DVD ISO");
        try {
            for (boolean fromIso : new boolean[]{false, true}) {
                String name = (reset ? "clock-reset-" : "demux-") + (fromIso ? "iso" : "folder");
                Path source = work.resolve(name + ".mkv");
                remux(engine, disc, source, plan, fromIso ? iso : 0);
                AdvancedMergerNative media = new AdvancedMergerNative();
                String[][] tracks = Arrays.stream(media.probe(source.toString()))
                    .map(row -> row.split("\t")).filter(f -> !f[1].equals("chapters")).toArray(String[][]::new);
                require(tracks.length == 3, "Expected MPEG2, AC3 and DVD subtitles");
                int[] indexes = Arrays.stream(tracks).mapToInt(f -> Integer.parseInt(f[0])).toArray();
                for (boolean vob : new boolean[]{false, true}) {
                    Path output = work.resolve(name + (vob ? "-vob" : "-elementary"));
                    Files.createDirectories(output);
                    engine.demuxProgress.clear();
                    directDemux(engine, disc, output, plan, fromIso ? iso : 0, vob, null, true);
                    require(engine.demuxProgress.contains(100) && engine.demuxProgress.stream().distinct().count() > 3, "No extraction progress: " + engine.demuxProgress);
                    for (String extension : new String[]{vob ? "VOB" : "mpeg2", "ac3", "sub", "idx"}) {
                        try (var files = Files.list(output)) {
                            Path file = files.filter(f -> f.toString().endsWith("." + extension)).findFirst().orElseThrow();
                            require(Files.size(file) > 0, "Empty DVD export: " + file);
                        }
                    }
                    String idx;
                    try (var files = Files.list(output)) { idx = Files.readString(files.filter(f -> f.toString().endsWith(".idx")).findFirst().orElseThrow()); }
                    require(idx.contains("size: 720x576") && idx.contains("palette:") && idx.contains("timestamp:"),
                        "Missing subtitle canvas, palette or timing: " + idx);
                    require(Files.readString(output.resolve("Chapters.txt")).contains("CHAPTER02="), "Lost DVD chapters");
                    if (!vob) {
                        int audioIndex;
                        try (var files = Files.list(output)) {
                            String filename = files.filter(f -> f.toString().endsWith(".ac3")).findFirst().orElseThrow().getFileName().toString();
                            audioIndex = Integer.parseInt(filename.substring(6, filename.indexOf('.')));
                        }
                        Path audioOnly = work.resolve(name + "-audio-only");
                        Files.createDirectories(audioOnly);
                        directDemux(engine, disc, audioOnly, plan, fromIso ? iso : 0, false, new int[]{audioIndex}, false);
                        try (var files = Files.list(audioOnly)) { require(files.count() == 1, "Unselected streams or chapters exported"); }
                        Path cancelled = work.resolve(name + "-cancelled");
                        Files.createDirectories(cancelled);
                        engine.cancelled = true;
                        try { directDemux(engine, disc, cancelled, plan, fromIso ? iso : 0, false, null, true); }
                        finally { engine.cancelled = false; }
                        try (var files = Files.list(cancelled)) { require(files.count() == 0, "Cancelled demux created output"); }
                    }
                }
            }
        } finally { engine.nativeCloseIso(iso); }
        System.out.println("Production JNI authored DVD folder/ISO demux, MPEG2/VOB, AC3, IDX/SUB and chapters PASS");
    }

    private static void remux(AndroidNativeRemuxEngine engine, Path disc, Path output, String[] plan, long iso) throws Exception {
        int titleSet = Integer.parseInt(plan[0].split("\t")[2]);
        List<Long> starts = new ArrayList<>(), ends = new ArrayList<>(), chapters = new ArrayList<>(), chapterEnds = new ArrayList<>();
        List<Integer> fds = new ArrayList<>();
        List<String> languages = new ArrayList<>();
        int[] palette = new int[16];
        for (String row : plan) {
            String[] fields = row.split("\t");
            if (fields[0].equals("C")) { starts.add(Long.parseLong(fields[1])); ends.add(Long.parseLong(fields[2])); }
            if (fields[0].equals("H")) { chapters.add(Long.parseLong(fields[1])); chapterEnds.add(Long.parseLong(fields[2])); }
            if (fields[0].equals("L")) languages.add(fields[1] + "\t" + fields[2]);
            if (fields[0].equals("P")) palette[Integer.parseInt(fields[1])] = Integer.parseInt(fields[2]);
        }
        require(!starts.isEmpty() && chapters.size() == 2, "Missing cells or main-movie chapters");
        int out = -1;
        try {
            if (iso == 0) for (int part = 1; part <= 9; part++) {
                Path file = disc.resolve(String.format(Locale.ROOT, "VIDEO_TS/VTS_%02d_%d.VOB", titleSet, part));
                if (!Files.exists(file)) break;
                fds.add(AndroidNativeRemuxEngine.openPath(file.toString(), false));
            }
            out = AndroidNativeRemuxEngine.openPath(output.toString(), true);
            String error = engine.nativeRemux(fds.stream().mapToInt(Integer::intValue).toArray(),
                starts.stream().mapToLong(Long::longValue).toArray(), ends.stream().mapToLong(Long::longValue).toArray(), out,
                chapters.stream().mapToLong(Long::longValue).toArray(), chapterEnds.stream().mapToLong(Long::longValue).toArray(),
                null, iso, titleSet, languages.toArray(new String[0]), palette);
            require(error == null, "Title remux failed: " + error);
        } finally {
            if (out >= 0) AndroidNativeRemuxEngine.closePath(out);
            for (int fd : fds) AndroidNativeRemuxEngine.closePath(fd);
        }
    }
    private static void directDemux(AndroidNativeRemuxEngine engine, Path disc, Path output, String[] plan, long iso, boolean vob, int[] selected, boolean includeChapters) throws Exception {
        int titleSet = Integer.parseInt(plan[0].split("\t")[2]);
        List<Long> starts = new ArrayList<>(), ends = new ArrayList<>(), chapters = new ArrayList<>(), chapterEnds = new ArrayList<>();
        List<Integer> fds = new ArrayList<>();
        List<String> languages = new ArrayList<>();
        int[] palette = new int[16];
        for (String row : plan) {
            String[] fields = row.split("\t");
            if (fields[0].equals("C")) { starts.add(Long.parseLong(fields[1])); ends.add(Long.parseLong(fields[2])); }
            if (fields[0].equals("H")) { chapters.add(Long.parseLong(fields[1])); chapterEnds.add(Long.parseLong(fields[2])); }
            if (fields[0].equals("L")) languages.add(fields[1] + "\t" + fields[2]);
            if (fields[0].equals("P")) palette[Integer.parseInt(fields[1])] = Integer.parseInt(fields[2]);
        }
        require(!starts.isEmpty() && chapters.size() == 2, "Missing cells or main-movie chapters");
        int out = -1;
        try {
            if (iso == 0) for (int part = 1; part <= 9; part++) {
                Path file = disc.resolve(String.format(Locale.ROOT, "VIDEO_TS/VTS_%02d_%d.VOB", titleSet, part));
                if (!Files.exists(file)) break;
                fds.add(AndroidNativeRemuxEngine.openPath(file.toString(), false));
            }
            String error = engine.nativeDemux(fds.stream().mapToInt(Integer::intValue).toArray(),
                starts.stream().mapToLong(Long::longValue).toArray(), ends.stream().mapToLong(Long::longValue).toArray(), output.toString(),
                includeChapters ? chapters.stream().mapToLong(Long::longValue).toArray() : new long[0], includeChapters ? chapterEnds.stream().mapToLong(Long::longValue).toArray() : new long[0],
                selected, iso, titleSet, languages.toArray(new String[0]), palette, vob);
            if (engine.cancelled) require(error != null && error.contains("cancelled"), "Cancellation ignored: " + error);
            else require(error == null, "Direct title demux failed: " + error);
        } finally {
            if (out >= 0) AndroidNativeRemuxEngine.closePath(out);
            for (int fd : fds) AndroidNativeRemuxEngine.closePath(fd);
        }
    }
}
