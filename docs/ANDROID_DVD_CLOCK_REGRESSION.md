# Android DVD clock reset and demux progress investigation

The reported device error was a Matroska write failure while submitting MPEG2
video with PTS 125, DTS 5, and time base 1/1000. Those values alone do not identify
the rejected packet: the interleaver can fail while flushing an earlier packet.
The original DVD has not been supplied. Mathias subsequently confirmed that
remux, batch mux, Advanced Merger and CLI mux all work in v1.4.17. Treat that
as the user-confirmed working baseline; the repeated-cell fixture below is a
separate defect and does not establish the cause of his v1.4.18 regression.

An independently authored DVD repeats a VOB in one title. The production
libdvdnav/libdvdread title planner returns both playback spans. The old JNI
remuxer concatenates the MPEG program streams without handling their clock
reset, and Matroska rejects non-monotonic DTS. Both v1.4.17 and v1.4.18's remux
code fail this reproduction. This establishes a defect, not proof that the
reported device failure was introduced by v1.4.18.

## Change

- Read NAV PCI/DSI with libdvdread and identify continuous clock segments.
  No MattMux IFO parser is introduced.
- Probe the complete title for track discovery, then isolate MPEG parser and
  GENPTS lookahead at clock boundaries. Preserve one common origin and apply
  NAV-derived offsets equally to video, audio, and subtitles. Do not clamp each
  stream independently or discard packets to hide timestamp errors.
- Keep stream identity across segments using DVD stream IDs and codec IDs.
- Report processed packet positions, rather than the read-ahead/probe cursor.
- Forward staging progress to the DVD tab and report native extraction progress.
  MKV staging reports percentage or copied MiB when its size is unknown.

## Regression coverage

The authored fixtures include continuous chapters and a repeated clock domain,
MPEG2 B-frames, AC3, DVD subtitles, and chapters. The production JNI tests cover
folder and UDF ISO input, elementary and VOB exports, exact packet payloads,
presentation timing, and intermediate extraction progress. NAV duration includes
the encoder's audio tail, so the repeated clock interval can be slightly longer
than the nominal six seconds. Tests require the same offset for every stream.
Android instrumentation additionally checks intermediate staging/extraction
statuses through TabMediaEngine and SAF. Existing sparse-PTS/B-frame,
folder/ISO parity, track-selection, and cancellation tests remain in place.

Windows and Linux already use FFmpeg's dvdvideo demuxer, which resets its MPEG
subdemuxer at NAV discontinuities and applies a common offset. The local FFmpeg
9.0.1 dvdvideo remux of the repeated-cell fixture produces the same interval.
Their media implementation therefore does not need the Android source adapter.
The PR's Windows and Linux workflows run the shared desktop behavior suites.

No public release is part of this change. Validation on the user's failing DVD
is still required before claiming that exact case is resolved.

## DVD-only input options (October 1 clarification)

Use analyzeduration=100000000 microseconds (100 seconds), probesize=100000000
bytes, and GENPTS on each original DVD input, including DVD audio selected in a
mixed Advanced Merger job. Do not apply them to its accompanying MKV, MP4, raw
streams, subtitle/chapter files, or to the staged MKV after DVD preparation.
Desktop ordinary-media input helpers previously applied all three options,
and Android merger_open enabled GENPTS for every ordinary input. These violated
the clarified policy and are corrected. DVD probing now also enables GENPTS.

In v1.4.17 Android demux stages the chosen DVD title into MKV using nativeRemux,
then extracts the chosen tracks from that MKV and copies the exports to SAF.
It is not a direct one-pass DVD-to-elementary-stream FFmpeg CLI command. The DVD
staging path already had both 100M limits and GENPTS in v1.4.17 and v1.4.18.
Those options were not removed between these releases. The production native
code difference was the DVD subtitle canvas metadata addition; the demux
export reader itself did not change between those two tags. This comparison
does not yet identify the original-device failure.

FFmpeg's concat-demuxer safe=0 accepts filenames rejected by safe=1. It does not
repair timestamps or suppress corruption errors; neither dvdvideo nor the
concat: byte-concatenation protocol uses this private demuxer option. GENPTS
fills missing presentation timestamps where decoding timestamps are available;
it does not promise to repair all existing timestamp discontinuities.
Source: https://ffmpeg.org/ffmpeg-formats.html (Format Options, concat, dvdvideo).

### Strict policy validation is blocked by raw MPEG-2

The strict no-GENPTS non-DVD candidate passes the new mixed-input option tests
and the native remux/demux suites. However, the unchanged desktop
TestMergerRawVideo/m2v and /vob cases fail with "Can't write packet with unknown
timestamp" after GENPTS is removed. This was independently reproduced using
FFmpeg 9.0.1 and the existing host fixtures (not just system FFmpeg 6.1.1):

```
ffmpeg -v error -i raw.m2v -map 0:v:0 -c copy output.mkv
# fails: Can't write packet with unknown timestamp
ffmpeg -v error -fflags +genpts -i raw.m2v -map 0:v:0 -c copy output.mkv
# succeeds
```

The same result occurs with raw.vob. No 100M probe overrides are needed for
these fixtures. The user's no-flags-for-raw-inputs rule has not been silently
relaxed. This candidate must remain draft/unmerged until that conflict is
resolved; the failing raw-video regression tests remain intact.
