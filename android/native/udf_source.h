#ifndef MATTMUX_UDF_SOURCE_H
#define MATTMUX_UDF_SOURCE_H
#include <stdint.h>
#include <stddef.h>
#include <udfread.h>
#include <blockinput.h>

typedef struct {
    udfread_block_input input; /* must be first */
    int fd;
    uint32_t blocks;
    udfread *volume;
    int (*cancelled)(void *);
    void *cancel_opaque;
} DvdUdfSource;

DvdUdfSource *dvd_udf_open(int fd, char *error, size_t error_size);
void dvd_udf_close(DvdUdfSource *source);
UDFFILE *dvd_udf_file(DvdUdfSource *source, int title_set, int part, int ifo);
#endif
