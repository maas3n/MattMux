package io.github.maas3n.mattmux;

/** Host harness invokes the production JNI entry points, not a second remuxer. */
public final class AndroidNativeRemuxEngine {
    private boolean cancelled;
    private int progress;
    private boolean isNativeCancelled() { return cancelled; }
    private void onNativeProgress(int value) { progress = value; }
    private native long[] nativeScanDvdNav(String path);
    private native String[] nativePlanDvdNav(String path, int title);
    private native long nativeOpenIso(int fd);
    private native void nativeCloseIso(long handle);
    private native byte[] nativeReadIsoIfo(long handle, int titleSet);
    private native String[] nativeProbeTracks(int[] fds, long[] starts, long[] ends, long iso, int titleSet, String[] languages, int[] palette);
    private native String nativeRemux(int[] fds, long[] starts, long[] ends, int output,
        long[] chapterStarts, long[] chapterEnds, int[] selectedStreams, long iso, int titleSet, String[] languages, int[] palette);
    private static native int openPath(String path, boolean output);
    private static native void closePath(int fd);

    public static void main(String[] args) throws Exception {
        System.load(args[0]);
        AndroidNativeRemuxEngine engine = new AndroidNativeRemuxEngine();
        String root = args[1];
        if (args.length > 2 && args[2].equals("dvdnav")) {
            testDvdNav(engine, root);
            return;
        }
        int first = openPath(root + "/VIDEO_TS/VTS_01_1.VOB", false);
        int second = openPath(root + "/VIDEO_TS/VTS_01_2.VOB", false);
        int isoFd = openPath(root + "/fixture.iso", false);
        long iso = engine.nativeOpenIso(isoFd);
        closePath(isoFd);
        try {
            byte[] expected = java.nio.file.Files.readAllBytes(java.nio.file.Path.of(root, "VIDEO_TS", "VIDEO_TS.IFO"));
            if (!java.util.Arrays.equals(expected, engine.nativeReadIsoIfo(iso, 0))) throw new AssertionError("IFO source mismatch");
            if (engine.nativeReadIsoIfo(iso, 99) != null) throw new AssertionError("Missing IFO accepted");
            long sectors = (java.nio.file.Files.size(java.nio.file.Path.of(root, "VIDEO_TS", "VTS_01_1.VOB")) +
                java.nio.file.Files.size(java.nio.file.Path.of(root, "VIDEO_TS", "VTS_01_2.VOB"))) / 2048;
            String[] languages = new String[]{"128\teng"};
            int[] palette = new int[16];
            String[] tracks = engine.nativeProbeTracks(new int[]{first, second}, new long[]{0}, new long[]{sectors}, 0, 1, languages, palette);
            String[] isoTracks = engine.nativeProbeTracks(new int[0], new long[]{0}, new long[]{sectors}, iso, 1, languages, palette);
            if (tracks == null || tracks.length < 2 || isoTracks == null || isoTracks.length != tracks.length) throw new AssertionError("Track probe mismatch");
            int videoIndex = -1;
            for (String track : tracks) {
                String[] fields = track.split("\t", -1);
                if (fields.length != 9) throw new AssertionError("Invalid track record: " + track);
                if (fields[1].equals("video")) videoIndex = Integer.parseInt(fields[0]);
                if (fields[1].equals("audio") && !fields[3].equals("eng")) throw new AssertionError("IFO audio language not applied: " + track);
            }
            if (videoIndex < 0) throw new AssertionError("No video stream in track metadata");
            for (boolean fromIso : new boolean[]{false, true}) {
                int output = openPath(root + (fromIso ? "/iso.mkv" : "/folder.mkv"), true);
                try {
                    String error = engine.nativeRemux(fromIso ? new int[0] : new int[]{first, second},
                        new long[]{0}, new long[]{sectors}, output,
                        new long[]{0, 1000}, new long[]{1000, 2000}, null, fromIso ? iso : 0, 1, languages, palette);
                    if (error != null) throw new AssertionError(error);
                    if (engine.progress != 100) throw new AssertionError("No completion progress");
                } finally { closePath(output); }
            }
            int selectedOutput = openPath(root + "/selected.mkv", true);
            try {
                String error = engine.nativeRemux(new int[]{first, second}, new long[]{0}, new long[]{sectors}, selectedOutput,
                    new long[]{0, 1000}, new long[]{1000, 2000}, new int[]{videoIndex}, 0, 1, languages, palette);
                if (error != null) throw new AssertionError(error);
            } finally { closePath(selectedOutput); }
            int output = openPath(root + "/cancelled.partial", true);
            try {
                engine.cancelled = true;
                String error = engine.nativeRemux(new int[]{first, second}, new long[]{0}, new long[]{sectors},
                    output, new long[]{0}, new long[]{2000}, null, 0, 1, languages, palette);
                if (error == null || !error.contains("cancelled")) throw new AssertionError("Cancellation ignored");
            } finally { closePath(output); }
        } finally {
            engine.nativeCloseIso(iso);
            closePath(first);
            closePath(second);
        }
        System.out.println("Production JNI folder/ISO remux and cancellation PASS");
    }
    private static void testDvdNav(AndroidNativeRemuxEngine engine, String root) throws Exception {
        long[] scan = engine.nativeScanDvdNav(root + "/staged");
        if (scan == null || scan.length != 5 || scan[0] != 2 || scan[1] != 2 ||
            scan[3] <= 0 || scan[4] <= scan[3]) {
            throw new AssertionError("Wrong one-based title discovery: " + java.util.Arrays.toString(scan));
        }
        long[] complete = engine.nativeScanDvdNav(root + "/disc");
        if (!java.util.Arrays.equals(scan, complete)) throw new AssertionError("IFO-only scan differs from full DVD");
        for (int title = 1; title <= 2; title++) {
            String[] plan = engine.nativePlanDvdNav(root + "/staged", title);
            if (plan == null || !plan[0].startsWith("T\t" + title + "\t")) throw new AssertionError("Cannot plan title " + title);
            long duration = Long.parseLong(plan[0].split("\t")[3]);
            if (duration != scan[title + 2] / 90) throw new AssertionError("Duration belongs to another title");
        }
        if (engine.nativePlanDvdNav(root + "/staged", 0) != null ||
            engine.nativePlanDvdNav(root + "/staged", 3) != null) throw new AssertionError("Invalid title accepted");
        String[] plan = engine.nativePlanDvdNav(root + "/staged", 2);
        int titleSet = Integer.parseInt(plan[0].split("\t")[2]);
        java.util.List<Long> starts = new java.util.ArrayList<>(), ends = new java.util.ArrayList<>();
        java.util.List<Long> chapterStarts = new java.util.ArrayList<>(), chapterEnds = new java.util.ArrayList<>();
        java.util.List<String> languages = new java.util.ArrayList<>();
        int[] palette = new int[16];
        for (String row : plan) {
            String[] parts = row.split("\t");
            switch (parts[0]) {
                case "C": starts.add(Long.parseLong(parts[1])); ends.add(Long.parseLong(parts[2])); break;
                case "H": chapterStarts.add(Long.parseLong(parts[1])); chapterEnds.add(Long.parseLong(parts[2])); break;
                case "L": languages.add(parts[1] + "\t" + parts[2]); break;
                case "P": palette[Integer.parseInt(parts[1])] = Integer.parseInt(parts[2]); break;
            }
        }
        if (chapterStarts.size() != 2 || starts.isEmpty()) throw new AssertionError("Missing chapters/cells");
        int vob = openPath(root + String.format("/disc/VIDEO_TS/VTS_%02d_1.VOB", titleSet), false);
        int isoFd = openPath(root + "/disc.iso", false);
        long iso = engine.nativeOpenIso(isoFd);
        closePath(isoFd);
        if (iso == 0) throw new AssertionError("Cannot open authored UDF ISO");
        try {
            // ISO metadata must produce the same native plan after staging.
            java.nio.file.Path isoStage = java.nio.file.Path.of(root, "iso-staged", "VIDEO_TS");
            java.nio.file.Files.createDirectories(isoStage);
            for (int set : new int[]{0, titleSet}) {
                byte[] bytes = engine.nativeReadIsoIfo(iso, set);
                if (bytes == null) throw new AssertionError("ISO IFO missing");
                java.nio.file.Files.write(isoStage.resolve(set == 0 ? "VIDEO_TS.IFO" : String.format("VTS_%02d_0.IFO", set)), bytes);
            }
            if (!java.util.Arrays.equals(plan, engine.nativePlanDvdNav(isoStage.getParent().toString(), 2)))
                throw new AssertionError("Folder/ISO plans differ");
            for (boolean fromIso : new boolean[]{false, true}) {
                int output = openPath(root + (fromIso ? "/iso.mkv" : "/folder.mkv"), true);
                try {
                    String error = engine.nativeRemux(fromIso ? new int[0] : new int[]{vob},
                        longs(starts), longs(ends), output, longs(chapterStarts), longs(chapterEnds), null,
                        fromIso ? iso : 0, titleSet, languages.toArray(new String[0]), palette);
                    if (error != null) throw new AssertionError(error);
                } finally { closePath(output); }
            }
        } finally { engine.nativeCloseIso(iso); closePath(vob); }
        System.out.println("Production DVDNav title 1/2, longest-title selection, native plans, chapters and folder/UDF ISO remux PASS");
    }

    private static long[] longs(java.util.List<Long> values) {
        return values.stream().mapToLong(Long::longValue).toArray();
    }

}
