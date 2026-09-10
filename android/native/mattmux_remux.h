#ifndef MATTMUX_REMUX_H
#define MATTMUX_REMUX_H

#include "mattmux_dvd.h"
#include <stdatomic.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef int (*mm_progress_fn)(void *opaque, int percent);

int mm_remux_title(
    mm_source *source,
    const mm_title_plan *plan,
    int output_fd,
    int preserve_chapters,
    _Atomic int *cancelled,
    mm_progress_fn progress,
    void *progress_opaque,
    int *out_stream_count,
    char *error,
    size_t error_size
);

#ifdef __cplusplus
}
#endif

#endif
