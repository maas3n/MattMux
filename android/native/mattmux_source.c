#define _FILE_OFFSET_BITS 64

#include "mattmux_dvd.h"

#include <errno.h>
#include <fcntl.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#include <sys/stat.h>
#include <unistd.h>

#include <udfread/blockinput.h>
#include <udfread/udfread.h>

#define MM_MAX_FILES 256

typedef struct {
    char *name;
    int fd;
    int64_t size;
} mm_named_fd;

typedef struct {
    udfread_block_input input;
    int fd;
    uint32_t blocks;
} mm_udf_input;

struct mm_source {
    mm_source_kind kind;
    mm_named_fd *files;
    size_t file_count;
    udfread *udf;
    mm_udf_input *udf_input;
};

struct mm_file {
    mm_source_kind kind;
    int fd;
    UDFFILE *udf_file;
    int64_t size;
};

static void set_error(char *error, size_t size, const char *fmt, ...)
{
    if (!error || size == 0) return;
    va_list ap;
    va_start(ap, fmt);
    vsnprintf(error, size, fmt, ap);
    va_end(ap);
}

static int64_t fd_size(int fd)
{
    struct stat st;
    if (fstat(fd, &st) == 0 && st.st_size > 0) return (int64_t)st.st_size;
    /* Some Storage Access Framework providers report st_size == 0 even for a
     * seekable document. Fall back to SEEK_END before declaring it empty. */
    off_t cur = lseek(fd, 0, SEEK_CUR);
    if (cur < 0) return -1;
    off_t end = lseek(fd, 0, SEEK_END);
    if (end < 0) return -1;
    (void)lseek(fd, cur, SEEK_SET);
    return (int64_t)end;
}

static int udf_input_close(udfread_block_input *input)
{
    (void)input;
    return 0;
}

static uint32_t udf_input_size(udfread_block_input *input)
{
    mm_udf_input *u = (mm_udf_input *)input;
    return u->blocks;
}

static int udf_input_read(
    udfread_block_input *input,
    uint32_t lba,
    void *buf,
    uint32_t nblocks,
    int flags
)
{
    (void)flags;
    mm_udf_input *u = (mm_udf_input *)input;
    if (!u || u->fd < 0 || !buf || nblocks == 0) return -1;

    uint64_t offset64 = (uint64_t)lba * MM_DVD_SECTOR_SIZE;
    uint64_t bytes64 = (uint64_t)nblocks * MM_DVD_SECTOR_SIZE;
    if (offset64 > INT64_MAX || bytes64 > SIZE_MAX) return -1;

    size_t bytes = (size_t)bytes64;
    size_t got = 0;
    while (got < bytes) {
        ssize_t n = pread(u->fd, (uint8_t *)buf + got, bytes - got, (off_t)(offset64 + got));
        if (n < 0) {
            if (errno == EINTR) continue;
            return got ? (int)(got / MM_DVD_SECTOR_SIZE) : -1;
        }
        if (n == 0) break;
        got += (size_t)n;
    }
    return (int)(got / MM_DVD_SECTOR_SIZE);
}

int mm_source_init_files(
    mm_source **out,
    const char *const *names,
    const int *fds,
    size_t count,
    char *error,
    size_t error_size
)
{
    if (!out || !names || !fds || count == 0 || count > MM_MAX_FILES) {
        set_error(error, error_size, "invalid VIDEO_TS file set");
        return -1;
    }

    mm_source *source = calloc(1, sizeof(*source));
    if (!source) {
        set_error(error, error_size, "out of memory");
        return -1;
    }
    source->kind = MM_SOURCE_FILES;
    source->files = calloc(count, sizeof(*source->files));
    if (!source->files) {
        free(source);
        set_error(error, error_size, "out of memory");
        return -1;
    }

    for (size_t i = 0; i < count; i++) {
        if (!names[i] || fds[i] < 0) {
            set_error(error, error_size, "invalid source file descriptor");
            mm_source_close(source);
            return -1;
        }
        source->files[i].name = strdup(names[i]);
        source->files[i].fd = dup(fds[i]);
        if (!source->files[i].name || source->files[i].fd < 0) {
            set_error(error, error_size, "could not duplicate source descriptor: %s", strerror(errno));
            mm_source_close(source);
            return -1;
        }
        source->files[i].size = fd_size(source->files[i].fd);
        if (source->files[i].size < 0) {
            set_error(error, error_size, "source provider does not expose a seekable file: %s", names[i]);
            mm_source_close(source);
            return -1;
        }
        source->file_count++;
    }

    *out = source;
    return 0;
}

int mm_source_init_iso(
    mm_source **out,
    int fd,
    char *error,
    size_t error_size
)
{
    if (!out || fd < 0) {
        set_error(error, error_size, "invalid ISO file descriptor");
        return -1;
    }

    mm_source *source = calloc(1, sizeof(*source));
    mm_udf_input *u = calloc(1, sizeof(*u));
    if (!source || !u) {
        free(source);
        free(u);
        set_error(error, error_size, "out of memory");
        return -1;
    }

    int dupfd = dup(fd);
    if (dupfd < 0) {
        free(source);
        free(u);
        set_error(error, error_size, "could not duplicate ISO descriptor: %s", strerror(errno));
        return -1;
    }
    int64_t size = fd_size(dupfd);
    if (size < MM_DVD_SECTOR_SIZE) {
        close(dupfd);
        free(source);
        free(u);
        set_error(error, error_size, "ISO source is empty or not seekable");
        return -1;
    }

    uint8_t probe;
    if (pread(dupfd, &probe, 1, 0) != 1) {
        close(dupfd);
        free(source);
        free(u);
        set_error(error, error_size,
                  "ISO provider is not seekable. Save the ISO locally and select it again.");
        return -1;
    }

    uint64_t blocks = ((uint64_t)size + MM_DVD_SECTOR_SIZE - 1) / MM_DVD_SECTOR_SIZE;
    if (blocks > UINT32_MAX) {
        close(dupfd);
        free(source);
        free(u);
        set_error(error, error_size, "ISO image is too large");
        return -1;
    }

    u->fd = dupfd;
    u->blocks = (uint32_t)blocks;
    u->input.close = udf_input_close;
    u->input.read = udf_input_read;
    u->input.size = udf_input_size;

    source->kind = MM_SOURCE_ISO;
    source->udf_input = u;
    source->udf = udfread_init();
    if (!source->udf) {
        mm_source_close(source);
        set_error(error, error_size, "could not initialize UDF reader");
        return -1;
    }
    if (udfread_open_input(source->udf, &u->input) < 0) {
        mm_source_close(source);
        set_error(error, error_size,
                  "ISO is not a readable UDF DVD image (encrypted, damaged, or unsupported)");
        return -1;
    }

    *out = source;
    return 0;
}

void mm_source_close(mm_source *source)
{
    if (!source) return;
    if (source->udf) {
        udfread_close(source->udf);
        source->udf = NULL;
    }
    if (source->udf_input) {
        if (source->udf_input->fd >= 0) close(source->udf_input->fd);
        free(source->udf_input);
    }
    if (source->files) {
        for (size_t i = 0; i < source->file_count; i++) {
            free(source->files[i].name);
            if (source->files[i].fd >= 0) close(source->files[i].fd);
        }
        free(source->files);
    }
    free(source);
}

static mm_named_fd *find_named_fd(mm_source *source, const char *name)
{
    if (!source || !name) return NULL;
    for (size_t i = 0; i < source->file_count; i++) {
        if (strcasecmp(source->files[i].name, name) == 0) return &source->files[i];
    }
    return NULL;
}

int mm_source_open_file(
    mm_source *source,
    const char *name,
    mm_file **out,
    char *error,
    size_t error_size
)
{
    if (!source || !name || !out) {
        set_error(error, error_size, "invalid DVD file request");
        return -1;
    }

    mm_file *file = calloc(1, sizeof(*file));
    if (!file) {
        set_error(error, error_size, "out of memory");
        return -1;
    }
    file->fd = -1;
    file->kind = source->kind;

    if (source->kind == MM_SOURCE_FILES) {
        mm_named_fd *entry = find_named_fd(source, name);
        if (!entry) {
            free(file);
            set_error(error, error_size, "%s was not found in the selected VIDEO_TS folder", name);
            return -1;
        }
        file->fd = dup(entry->fd);
        file->size = entry->size;
        if (file->fd < 0) {
            free(file);
            set_error(error, error_size, "could not open %s: %s", name, strerror(errno));
            return -1;
        }
    } else {
        char path[96];
        snprintf(path, sizeof(path), "/VIDEO_TS/%s", name);
        file->udf_file = udfread_file_open(source->udf, path);
        if (!file->udf_file) {
            snprintf(path, sizeof(path), "/video_ts/%s", name);
            file->udf_file = udfread_file_open(source->udf, path);
        }
        if (!file->udf_file) {
            free(file);
            set_error(error, error_size, "%s was not found inside the ISO", name);
            return -1;
        }
        file->size = udfread_file_size(file->udf_file);
        if (file->size < 0) {
            udfread_file_close(file->udf_file);
            free(file);
            set_error(error, error_size, "could not determine size of %s inside ISO", name);
            return -1;
        }
    }

    *out = file;
    return 0;
}

int64_t mm_file_size(mm_file *file)
{
    return file ? file->size : -1;
}

ssize_t mm_file_read_at(mm_file *file, void *buf, size_t size, int64_t offset)
{
    if (!file || !buf || offset < 0) return -1;
    if (offset >= file->size) return 0;
    if ((uint64_t)size > (uint64_t)(file->size - offset)) size = (size_t)(file->size - offset);

    if (file->kind == MM_SOURCE_FILES) {
        size_t got = 0;
        while (got < size) {
            ssize_t n = pread(file->fd, (uint8_t *)buf + got, size - got, (off_t)(offset + (int64_t)got));
            if (n < 0) {
                if (errno == EINTR) continue;
                return got ? (ssize_t)got : -1;
            }
            if (n == 0) break;
            got += (size_t)n;
        }
        return (ssize_t)got;
    }

    if (udfread_file_seek(file->udf_file, offset, UDF_SEEK_SET) < 0) return -1;
    return udfread_file_read(file->udf_file, buf, size);
}

void mm_file_close(mm_file *file)
{
    if (!file) return;
    if (file->fd >= 0) close(file->fd);
    if (file->udf_file) udfread_file_close(file->udf_file);
    free(file);
}
