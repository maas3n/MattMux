# Advanced Merger

Advanced Merger combines selected video, audio, subtitle, and optional chapter data into a new Matroska (`.mkv`) file without transcoding.

The feature is available from the **Advanced Merger** tab. It is separate from the DVD-title remux workflow.

## Inputs

- **CHOOSE MOVIE FILES** accepts containers and elementary video files understood by the bundled FFmpeg runtime. Only video streams discovered in these inputs are added to **Select Streams**.
- **CHOOSE AUDIO FILES** accepts containers and elementary audio files understood by FFmpeg. Only audio streams are added to **Select Streams**.
- **CHOOSE SUBTITLE FILES** accepts containers and subtitle files understood by FFmpeg. Only subtitle streams are added to **Select Streams**.
- **CHOOSE CHAPTER FILE** accepts one FFmetadata chapter file. It must start with `;FFMETADATA1` and contain valid chapters.

A source container may contain many kinds of streams. The button used to add it determines which stream category MattMux exposes. For example, adding the same MKV through **CHOOSE AUDIO FILES** exposes its audio tracks but not its video or subtitle tracks.

Raw/elementary inputs are supported when the bundled FFmpeg runtime can demux them, including common H.264, MPEG-2/VOB, AAC, AC-3, MP3, DTS, SRT, WebVTT, SUP and similar formats.

For VobSub on Android/ChromeOS, select the matching `.idx` and `.sub` files together so MattMux can stage the sidecar pair before probing.

## Stream selection and output

Every discovered matching stream is initially selected. Clear any checkbox you do not want in the output, choose the output folder, enter an `.mkv` filename, and press **MUX TO MKV**.

MattMux maps the exact selected input stream indexes and uses stream copy (`-c copy` on desktop, equivalent native libav packet copying on Android/ChromeOS). It does not intentionally re-encode video or audio. Desktop inputs use generated presentation timestamps (`-fflags +genpts`) before muxing.

Existing output files are not overwritten. Cancel stops the active probe/copy/mux operation and partial operation-owned output is not promoted to the requested final filename.

## Android / ChromeOS temporary storage

Android's Storage Access Framework does not guarantee a normal seekable filesystem path for every selected document. MattMux therefore stages selected Advanced Merger inputs in private temporary storage and creates the output there before copying the completed MKV to the chosen destination.

The device needs enough free temporary space for the selected input copies plus the in-progress output. Temporary merger data is cleaned up when the panel is destroyed or an operation completes.

## Example mapping

A selection equivalent to this FFmpeg command can be built through the UI:

```text
ffmpeg -i input.mp4 -i input.mkv -i input.ac3 -i input.srt \
  -map 1:v -map 0:a -map 2:a -map 1:s:0 -map 3:s \
  -c copy output.mkv
```

MattMux constructs its maps from the individual stream checkboxes rather than mapping every stream of a category automatically.
