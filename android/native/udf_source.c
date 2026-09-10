#define _POSIX_C_SOURCE 200809L
#include "udf_source.h"
#include <errno.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/stat.h>
#include <unistd.h>

static uint32_t block_count(udfread_block_input *input)
{
    return ((DvdUdfSource *)input)->blocks;
}

static int read_blocks(udfread_block_input *input, uint32_t lba, void *buf,
                       uint32_t count, int flags)
{
    (void)flags;
    DvdUdfSource *source = (DvdUdfSource *)input;
    if (count > INT_MAX || lba > source->blocks || count > source->blocks - lba) return -1;
    size_t bytes = (size_t)count * UDF_BLOCK_SIZE;
    size_t done = 0;
    while (done < bytes) {
        if (source->cancelled && source->cancelled(source->cancel_opaque)) return -1;
        ssize_t n = pread(source->fd, (char *)buf + done, bytes - done,
                          (off_t)lba * UDF_BLOCK_SIZE + (off_t)done);
        if (n < 0 && errno == EINTR) continue;
        if (n <= 0) return -1; /* short media reads are errors, never padded */
        done += (size_t)n;
    }
    return (int)count;
}

DvdUdfSource *dvd_udf_open(int fd, char *error, size_t error_size)
{
    struct stat st;
    unsigned char probe;
    if (fstat(fd, &st) || st.st_size < 257LL * UDF_BLOCK_SIZE ||
        st.st_size % UDF_BLOCK_SIZE || st.st_size / UDF_BLOCK_SIZE > UINT32_MAX ||
        pread(fd, &probe, 1, 0) != 1) {
        snprintf(error, error_size, "ISO must be a seekable, complete UDF image. Copy it to local storage and select it again.");
        return NULL;
    }
    DvdUdfSource *source = calloc(1, sizeof(*source));
    if (!source) return NULL;
    source->fd = dup(fd);
    source->input.read = read_blocks;
    source->input.size = block_count;
    source->blocks = (uint32_t)(st.st_size / UDF_BLOCK_SIZE);
    source->volume = udfread_init();
    if (source->fd < 0 || !source->volume || udfread_open_input(source->volume, &source->input) < 0) {
        snprintf(error, error_size, "Could not open UDF filesystem. The image may be truncated, corrupt, or ISO9660-only.");
        dvd_udf_close(source);
        return NULL;
    }
    return source;
}

void dvd_udf_close(DvdUdfSource *source)
{
    if (!source) return;
    if (source->volume) udfread_close(source->volume);
    if (source->fd >= 0) close(source->fd);
    free(source);
}

UDFFILE *dvd_udf_file(DvdUdfSource *source, int title_set, int part, int ifo)
{
    char path[64];
    if (!source || title_set < 0 || title_set > 99 || part < 0 || part > 9) return NULL;
    if (ifo && title_set == 0) snprintf(path, sizeof(path), "/VIDEO_TS/VIDEO_TS.IFO");
    else snprintf(path, sizeof(path), "/VIDEO_TS/VTS_%02d_%d.%s", title_set, part, ifo ? "IFO" : "VOB");
    return udfread_file_open(source->volume, path);
}
