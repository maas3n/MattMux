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
        System.out.println("Production JNI longest/explicit title selection, staged folder/ISO planning and remux PASS");
    }

    private static void remux(AndroidNativeRemuxEngine engine, Path disc, Path output, String[] plan, long iso) throws Exception {
        int titleSet = Integer.parseInt(plan[0].split("\t")[2]);
        List<Long> starts = new ArrayList<>(), ends = new ArrayList<>(), chapters = new ArrayList<>(), chapterEnds = new ArrayList<>();
        List<Integer> fds = new ArrayList<>();
        List<String> languages = new ArrayList<>();
        for (String row : plan) {
            String[] fields = row.split("\t");
            if (fields[0].equals("C")) { starts.add(Long.parseLong(fields[1])); ends.add(Long.parseLong(fields[2])); }
            if (fields[0].equals("H")) { chapters.add(Long.parseLong(fields[1])); chapterEnds.add(Long.parseLong(fields[2])); }
            if (fields[0].equals("L")) languages.add(fields[1] + "\t" + fields[2]);
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
                null, iso, titleSet, languages.toArray(new String[0]), new int[16]);
            require(error == null, "Title remux failed: " + error);
        } finally {
            if (out >= 0) AndroidNativeRemuxEngine.closePath(out);
            for (int fd : fds) AndroidNativeRemuxEngine.closePath(fd);
        }
    }
}
