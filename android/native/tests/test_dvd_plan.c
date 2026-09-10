#include "../mattmux_dvd.h"

#include <assert.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

static void put16(uint8_t *p, uint16_t v) { p[0]=(uint8_t)(v>>8); p[1]=(uint8_t)v; }
static void put32(uint8_t *p, uint32_t v) { p[0]=(uint8_t)(v>>24); p[1]=(uint8_t)(v>>16); p[2]=(uint8_t)(v>>8); p[3]=(uint8_t)v; }

static void set_cell(uint8_t *cell, uint8_t seconds_bcd, uint8_t flags, uint32_t first, uint32_t last)
{
    memset(cell, 0, 24);
    cell[0] = flags;
    cell[6] = seconds_bcd;
    cell[7] = 0xC0;
    put32(cell + 8, first);
    put32(cell + 12, first);
    put32(cell + 16, last);
    put32(cell + 20, last);
}

static int write_temp(const uint8_t *data, size_t size)
{
    char path[] = "/tmp/mattmux-ifo-XXXXXX";
    int fd = mkstemp(path);
    assert(fd >= 0);
    unlink(path);
    assert(write(fd, data, size) == (ssize_t)size);
    assert(lseek(fd, 0, SEEK_SET) == 0);
    return fd;
}

static mm_source *make_source(int angle)
{
    uint8_t *vmg = calloc(1, 2 * MM_DVD_SECTOR_SIZE);
    uint8_t *vts = calloc(1, 3 * MM_DVD_SECTOR_SIZE);
    assert(vmg && vts);

    memcpy(vmg, "DVDVIDEO-VMG", 12);
    put32(vmg + 0xC4, 1);
    size_t tt = MM_DVD_SECTOR_SIZE;
    put16(vmg + tt, 1);
    put32(vmg + tt + 4, 19);
    size_t entry = tt + 8;
    put16(vmg + entry + 2, angle ? 2 : 3);
    vmg[entry + 6] = 1;
    vmg[entry + 7] = 1;

    memcpy(vts, "DVDVIDEO-VTS", 12);
    put32(vts + 0xC8, 1);
    put32(vts + 0xCC, 2);
    size_t ptt = MM_DVD_SECTOR_SIZE;
    put16(vts + ptt, 1);
    if (!angle) {
        put32(vts + ptt + 4, 23);
        put32(vts + ptt + 8, 12);
        for (int i = 0; i < 3; i++) {
            put16(vts + ptt + 12 + i * 4, 1);
            put16(vts + ptt + 14 + i * 4, (uint16_t)(i + 1));
        }
    } else {
        put32(vts + ptt + 4, 19);
        put32(vts + ptt + 8, 12);
        put16(vts + ptt + 12, 1); put16(vts + ptt + 14, 1);
        put16(vts + ptt + 16, 1); put16(vts + ptt + 18, 2);
    }

    size_t pgci = 2 * MM_DVD_SECTOR_SIZE;
    put16(vts + pgci, 1);
    const uint32_t pgc_rel = 16;
    const int programs = angle ? 2 : 3;
    const int cells = 3;
    const uint32_t pgc_size = 0xF0 + cells * 24;
    put32(vts + pgci + 4, pgc_rel + pgc_size - 1);
    put32(vts + pgci + 12, pgc_rel);
    size_t pgc = pgci + pgc_rel;
    vts[pgc + 2] = (uint8_t)programs;
    vts[pgc + 3] = (uint8_t)cells;
    put16(vts + pgc + 0xE6, 0xEC);
    put16(vts + pgc + 0xE8, 0xF0);
    if (!angle) {
        vts[pgc + 0xEC] = 1; vts[pgc + 0xED] = 2; vts[pgc + 0xEE] = 3;
        set_cell(vts + pgc + 0xF0 + 0 * 24, 0x10, 0x00, 0, 9);
        set_cell(vts + pgc + 0xF0 + 1 * 24, 0x20, 0x00, 10, 29);
        set_cell(vts + pgc + 0xF0 + 2 * 24, 0x30, 0x00, 30, 59);
    } else {
        vts[pgc + 0xEC] = 1; vts[pgc + 0xED] = 3;
        set_cell(vts + pgc + 0xF0 + 0 * 24, 0x05, 0x40, 0, 4);
        set_cell(vts + pgc + 0xF0 + 1 * 24, 0x07, 0xC0, 5, 11);
        set_cell(vts + pgc + 0xF0 + 2 * 24, 0x10, 0x00, 12, 21);
    }

    int fds[2] = { write_temp(vmg, 2 * MM_DVD_SECTOR_SIZE), write_temp(vts, 3 * MM_DVD_SECTOR_SIZE) };
    const char *names[2] = {"VIDEO_TS.IFO", "VTS_01_0.IFO"};
    char error[MM_MAX_ERROR] = {0};
    mm_source *source = NULL;
    assert(mm_source_init_files(&source, names, fds, 2, error, sizeof(error)) == 0);
    close(fds[0]); close(fds[1]);
    free(vmg); free(vts);
    return source;
}

static void test_simple(void)
{
    mm_source *source = make_source(0);
    mm_title_plan plan;
    char error[MM_MAX_ERROR] = {0};
    assert(mm_build_title_plan(source, 1, &plan, error, sizeof(error)) == 0);
    assert(plan.title_set == 1);
    assert(plan.chapter_count == 3);
    assert(plan.duration_ms == 60000);
    assert(plan.chapters[0].start_ms == 0 && plan.chapters[0].duration_ms == 10000);
    assert(plan.chapters[1].start_ms == 10000 && plan.chapters[1].duration_ms == 20000);
    assert(plan.chapters[2].start_ms == 30000 && plan.chapters[2].duration_ms == 30000);
    assert(plan.cell_count == 3);
    assert(plan.cells[0].first_sector == 0 && plan.cells[0].last_sector == 9);
    assert(plan.cells[2].first_sector == 30 && plan.cells[2].last_sector == 59);
    mm_title_plan_free(&plan);
    mm_source_close(source);
}

static void test_angle_one(void)
{
    mm_source *source = make_source(1);
    mm_title_plan plan;
    char error[MM_MAX_ERROR] = {0};
    assert(mm_build_title_plan(source, 1, &plan, error, sizeof(error)) == 0);
    assert(plan.chapter_count == 2);
    assert(plan.duration_ms == 15000);
    assert(plan.cell_count == 2);
    assert(plan.cells[0].first_sector == 0 && plan.cells[0].last_sector == 4);
    assert(plan.cells[1].first_sector == 12 && plan.cells[1].last_sector == 21);
    mm_title_plan_free(&plan);
    mm_source_close(source);
}

int main(void)
{
    test_simple();
    test_angle_one();
    puts("native DVD plan parity tests passed");
    return 0;
}
