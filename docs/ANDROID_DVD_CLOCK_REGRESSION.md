# Android DVD clock reset and demux progress investigation

The reported device error was a Matroska write failure while submitting MPEG2
video with PTS 125, DTS 5, and time base 1/1000. Those values alone do not identify
the rejected packet: the interleaver can fail while flushing an earlier packet.
The original DVD and the last working APK have not been supplied.

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
