#define _POSIX_C_SOURCE 200809L
#include "udf_source.h"
#include <assert.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static int cancel_read(void *unused) { (void)unused; return 1; }

static void compare_file(DvdUdfSource *source, const char *folder, int title, int part, int ifo)
{
    char path[512];
    if (!title) snprintf(path, sizeof(path), "%s/VIDEO_TS.IFO", folder);
    else snprintf(path, sizeof(path), "%s/VTS_%02d_%d.%s", folder, title, part, ifo ? "IFO" : "VOB");
    int fd = open(path, O_RDONLY);
    assert(fd >= 0);
    UDFFILE *file = dvd_udf_file(source, title, part, ifo);
    assert(file);
    off_t size = lseek(fd, 0, SEEK_END);
    assert(size == udfread_file_size(file));
    unsigned char expected[4097], actual[4097];
    /* Unaligned reads and backward seeks exercise the public UDF file API. */
    for (off_t offset = size - 1; offset >= 0; offset -= 997) {
        ssize_t want = pread(fd, expected, sizeof(expected), offset);
        assert(want > 0);
        assert(udfread_file_seek(file, offset, SEEK_SET) == offset);
        assert(udfread_file_read(file, actual, (size_t)want) == want);
        assert(!memcmp(expected, actual, (size_t)want));
    }
    assert(udfread_file_seek(file, 0, SEEK_SET) == 0);
    for (off_t offset = 0; offset < size;) {
        ssize_t want = pread(fd, expected, sizeof(expected), offset);
        assert(want > 0);
        assert(udfread_file_read(file, actual, (size_t)want) == want);
        assert(!memcmp(expected, actual, (size_t)want));
        offset += want;
    }
    assert(udfread_file_read(file, actual, 1) == 0);
    udfread_file_close(file);
    close(fd);
}

int main(int argc, char **argv)
{
    assert(argc == 3);
    char error[256];
    int fd = open(argv[1], O_RDONLY);
    assert(fd >= 0);
    DvdUdfSource *source = dvd_udf_open(fd, error, sizeof(error));
    if (!source) { fprintf(stderr, "%s\n", error); return 1; }
    close(fd); /* the reader must own its duplicate */
    compare_file(source, argv[2], 0, 0, 1);
    compare_file(source, argv[2], 1, 0, 1);
    compare_file(source, argv[2], 1, 1, 0);
    compare_file(source, argv[2], 1, 2, 0);
    assert(!dvd_udf_file(source, 2, 0, 1));
    unsigned char block[2048];
    assert(source->input.read(&source->input, source->blocks, block, 1, 0) < 0);
    source->cancelled = cancel_read;
    assert(source->input.read(&source->input, 0, block, 1, 0) < 0);
    dvd_udf_close(source);
    int pipes[2];
    assert(!pipe(pipes));
    assert(!dvd_udf_open(pipes[0], error, sizeof(error)));
    close(pipes[0]); close(pipes[1]);
    fd = open("/dev/null", O_RDONLY);
    assert(!dvd_udf_open(fd, error, sizeof(error)));
    close(fd);
    puts("UDF source parity: sequential bytes, unaligned/backward seeks, missing files, fd ownership, bounds, cancellation and pipes PASS");
    return 0;
}
