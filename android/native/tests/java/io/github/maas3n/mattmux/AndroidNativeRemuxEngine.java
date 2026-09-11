package io.github.maas3n.mattmux;

/** Host harness invokes the production JNI entry points, not a second remuxer. */
public final class AndroidNativeRemuxEngine {
    private boolean cancelled;
    private int progress;
    private boolean isNativeCancelled() { return cancelled; }
    private void onNativeProgress(int value) { progress = value; }
    private native long nativeOpenIso(int fd);
    private native void nativeCloseIso(long handle);
    private native byte[] nativeReadIsoIfo(long handle, int titleSet);
    private native String nativeRemux(int[] fds, long[] starts, long[] ends, int output,
        long[] chapterStarts, long[] chapterEnds, long iso, int titleSet);
    private static native int openPath(String path, boolean output);
    private static native void closePath(int fd);

    public static void main(String[] args) throws Exception {
        System.load(args[0]);
        AndroidNativeRemuxEngine engine = new AndroidNativeRemuxEngine();
        String root = args[1];
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
            for (boolean fromIso : new boolean[]{false, true}) {
                int output = openPath(root + (fromIso ? "/iso.mkv" : "/folder.mkv"), true);
                try {
                    String error = engine.nativeRemux(fromIso ? new int[0] : new int[]{first, second},
                        new long[]{0}, new long[]{sectors}, output,
                        new long[]{0, 1000}, new long[]{1000, 2000}, fromIso ? iso : 0, 1);
                    if (error != null) throw new AssertionError(error);
                    if (engine.progress != 100) throw new AssertionError("No completion progress");
                } finally { closePath(output); }
            }
            int output = openPath(root + "/cancelled.partial", true);
            try {
                engine.cancelled = true;
                String error = engine.nativeRemux(new int[]{first, second}, new long[]{0}, new long[]{sectors},
                    output, new long[]{0}, new long[]{2000}, 0, 1);
                if (error == null || !error.contains("cancelled")) throw new AssertionError("Cancellation ignored");
            } finally { closePath(output); }
        } finally {
            engine.nativeCloseIso(iso);
            closePath(first);
            closePath(second);
        }
        System.out.println("Production JNI folder/ISO remux and cancellation PASS");
    }
}
