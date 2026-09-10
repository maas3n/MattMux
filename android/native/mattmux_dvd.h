#ifndef MATTMUX_DVD_H
#define MATTMUX_DVD_H

#include <stddef.h>
#include <stdint.h>
#include <sys/types.h>

#ifdef __cplusplus
extern "C" {
#endif

#define MM_DVD_SECTOR_SIZE 2048
#define MM_MAX_ERROR 1024

typedef enum {
    MM_SOURCE_FILES = 0,
    MM_SOURCE_ISO = 1,
} mm_source_kind;

typedef struct mm_source mm_source;
typedef struct mm_file mm_file;

typedef struct {
    int number;
    int64_t duration_ms;
    int chapter_count;
} mm_title_summary;

typedef struct {
    int number;
    int64_t start_ms;
    int64_t duration_ms;
} mm_chapter;

typedef struct {
    uint32_t first_sector;
    uint32_t last_sector;
    int64_t timeline_start_ms;
    int64_t duration_ms;
} mm_cell;

typedef struct {
    int number;
    int title_set;
    int title_set_title;
    int pgcn;
    int64_t duration_ms;
    mm_chapter *chapters;
    size_t chapter_count;
    mm_cell *cells;
    size_t cell_count;
} mm_title_plan;

int mm_source_init_files(
    mm_source **out,
    const char *const *names,
    const int *fds,
    size_t count,
    char *error,
    size_t error_size
);

int mm_source_init_iso(
    mm_source **out,
    int fd,
    char *error,
    size_t error_size
);

void mm_source_close(mm_source *source);

int mm_source_open_file(
    mm_source *source,
    const char *name,
    mm_file **out,
    char *error,
    size_t error_size
);

int64_t mm_file_size(mm_file *file);
ssize_t mm_file_read_at(mm_file *file, void *buf, size_t size, int64_t offset);
void mm_file_close(mm_file *file);

int mm_scan_titles(
    mm_source *source,
    mm_title_summary **out_titles,
    size_t *out_count,
    char *error,
    size_t error_size
);

int mm_build_title_plan(
    mm_source *source,
    int global_title,
    mm_title_plan *out,
    char *error,
    size_t error_size
);

void mm_title_plan_free(mm_title_plan *plan);

#ifdef __cplusplus
}
#endif

#endif
