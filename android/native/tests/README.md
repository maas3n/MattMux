# Alpha 4 source and remux checks

`test-udf-source.sh <pinned-libudfread-source>` compares complete file bytes and
unaligned/backward reads against generated originals under ASan/UBSan. It checks
fd duplication, missing files, input bounds, cancellation and non-seekable input.
The same originals are wrapped in UDF 2.60 (pycdlib) and UDF 1.02 (genisoimage).
These are filesystem fixtures, not DVD-Video authoring/IFO fixtures.

`test-host-remux.sh <pinned-libudfread-source> <diagnostic-directory>` compiles the
**production JNI source** on Linux with the host libav libraries and invokes it
from Java. An original two-second PAL MPEG-2/AC-3 stream is split across two VOBs
and read through both backends. Fingerprints compare compressed packet hashes,
stream properties, common-clock PTS/DTS and chapters. The harness supplies cell
and chapter arrays directly; it does not claim to test real IFO navigation.
The host-only test library is never packaged in the Android APK.

`DvdIfoParserTest` separately verifies IFO selection, angle-1 cell ranges, chapters,
BCD validation and stable `diagnosticJson()`. The app logs the selected plan under
`MattMuxPlan`; successful `RemuxResult` also carries it as `planJson`.

## Remaining equivalence gates

These tests do not establish Linux/Windows release equivalence. Before claiming
that, add independently authored DVD-Video/UDF 1.02 fixtures and golden plans for
multiple titles, shared PGCs, fragmented files, multiple languages/subtitles and
cell clock discontinuities. Run actual Android JNI on both ABIs, compare against
the desktop dvdvideo engines, and test real SAF providers on Chromebooks.

Use a common timestamp origin across all streams, so an A/V offset cannot be
hidden by per-stream normalization. MPEG B-frame PTS may legitimately be
non-monotonic in demux order; check DTS and presentation order appropriately.
Only introduce measured, documented cross-muxer tolerances. Do not weaken the
exact folder/ISO test to accommodate a mismatch.

Interleaved multi-angle input currently fails closed: IFO cell spans alone do
not select VOBUs safely. No full parity or production-readiness claim is made.
