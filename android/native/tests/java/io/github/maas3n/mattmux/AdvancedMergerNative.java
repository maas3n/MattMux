package io.github.maas3n.mattmux;

public final class AdvancedMergerNative {
    private boolean cancelled;
    private boolean isNativeCancelled() { return cancelled; }
    private native String[] probe(String path);
    private native String validateChapters(String path);
    private native String mux(String[] paths, int[] sources, int[] streams, String chapters, String output);
    public static void main(String[] args) {
        System.load(args[0]);
        AdvancedMergerNative engine = new AdvancedMergerNative();
        String dir = args[1];
        String[] tracks = engine.probe(dir + "/mixed.mkv");
        if (tracks.length != 5) throw new AssertionError("Expected 2 video, 2 audio, 1 subtitle");
        if (engine.validateChapters(dir + "/chapters.txt") != null) throw new AssertionError("Valid chapters rejected");
        if (engine.validateChapters(dir + "/captions.srt") == null) throw new AssertionError("Invalid chapters accepted");
        String[] paths = {dir + "/mixed.mkv", dir + "/raw.ac3", dir + "/captions.srt"};
        String error = engine.mux(paths, new int[]{0,0,0,1,2}, new int[]{1,3,4,0,0}, dir + "/chapters.txt", dir + "/merged.mkv");
        if (error != null) throw new AssertionError(error);
        error = engine.mux(paths, new int[]{0}, new int[]{1}, null, dir + "/no-chapters.mkv");
        if (error != null) throw new AssertionError(error);
        error = engine.mux(paths, new int[]{0}, new int[]{99}, null, dir + "/invalid.mkv");
        if (error == null) throw new AssertionError("Invalid stream index accepted");
        engine.cancelled = true;
        error = engine.mux(paths, new int[]{0}, new int[]{1}, null, dir + "/cancelled.mkv");
        if (error == null) throw new AssertionError("Cancellation ignored");
        engine.cancelled = false;
        for (String ext : new String[]{"h264", "m2v", "vob"}) {
            error = engine.mux(new String[]{dir + "/raw." + ext}, new int[]{0}, new int[]{0}, null, dir + "/raw-" + ext + ".mkv");
            if (error != null) throw new AssertionError(ext + ": " + error);
        }
    }
}
