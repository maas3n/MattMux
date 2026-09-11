#include <jni.h>
#include <errno.h>
#include <limits.h>
#include "udf_source.h"
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>
#include <libavutil/dict.h>
#include <libavutil/error.h>
#include <libavutil/mathematics.h>
#include <libavutil/channel_layout.h>
#include <libavutil/mem.h>

#define IO_BUFFER_SIZE (64 * 1024)
#define DVD_SECTOR_SIZE 2048LL

typedef struct { JNIEnv *env; jobject engine; jmethodID method; } CancelContext;

static int is_cancelled(void *opaque)
{
    CancelContext *ctx = opaque;
    if (!ctx || !ctx->method) return 0;
    jboolean result = (*ctx->env)->CallBooleanMethod(ctx->env, ctx->engine, ctx->method);
    return (*ctx->env)->ExceptionCheck(ctx->env) || result;
}

typedef struct {
    int64_t source_start;
    int64_t length;
    int64_t stream_start;
} SourceSpan;

typedef struct {
    CancelContext *cancel;
    DvdUdfSource *udf; /* borrowed for this JNI call */
    UDFFILE *udf_files[9];
    int *fds;
    int fd_count;
    int64_t *file_starts;
    int64_t total_source_size;
    SourceSpan *spans;
    int span_count;
    int64_t stream_size;
    int64_t pos;
} SourceContext;

typedef struct {
    int fd;
    CancelContext *cancel;
    int64_t pos;
} OutputContext;

static void format_version(char *out, size_t out_size, unsigned version)
{
    snprintf(out, out_size, "%u.%u.%u",
        (version >> 16) & 0xff,
        (version >> 8) & 0xff,
        version & 0xff);
}

static void ff_error(char *out, size_t out_size, const char *step, int err)
{
    char detail[AV_ERROR_MAX_STRING_SIZE] = {0};
    av_strerror(err, detail, sizeof(detail));
    snprintf(out, out_size, "%s: %s", step, detail);
}

static int find_span(const SourceContext *ctx, int64_t stream_pos)
{
    for (int i = 0; i < ctx->span_count; ++i) {
        const int64_t start = ctx->spans[i].stream_start;
        const int64_t end = start + ctx->spans[i].length;
        if (stream_pos >= start && stream_pos < end) return i;
    }
    return -1;
}

static int find_file(const SourceContext *ctx, int64_t source_pos)
{
    for (int i = 0; i < ctx->fd_count; ++i) {
        const int64_t start = ctx->file_starts[i];
        const int64_t end = (i + 1 < ctx->fd_count) ? ctx->file_starts[i + 1] : ctx->total_source_size;
        if (source_pos >= start && source_pos < end) return i;
    }
    return -1;
}

static int source_read(void *opaque, uint8_t *buf, int buf_size)
{
    SourceContext *ctx = (SourceContext *)opaque;
    if (is_cancelled(ctx->cancel)) return AVERROR_EXIT;
    if (ctx->pos >= ctx->stream_size) return AVERROR_EOF;

    int span_index = find_span(ctx, ctx->pos);
    if (span_index < 0) return AVERROR(EIO);
    SourceSpan *span = &ctx->spans[span_index];
    int64_t in_span = ctx->pos - span->stream_start;
    int64_t source_pos = span->source_start + in_span;
    int file_index = find_file(ctx, source_pos);
    if (file_index < 0) return AVERROR(EIO);

    int64_t file_start = ctx->file_starts[file_index];
    int64_t file_end = (file_index + 1 < ctx->fd_count) ? ctx->file_starts[file_index + 1] : ctx->total_source_size;
    int64_t remaining_span = span->length - in_span;
    int64_t remaining_file = file_end - source_pos;
    int64_t wanted = buf_size;
    if (wanted > remaining_span) wanted = remaining_span;
    if (wanted > remaining_file) wanted = remaining_file;
    if (wanted <= 0) return AVERROR(EIO);

    ssize_t n;
    if (ctx->udf) {
        UDFFILE *file = ctx->udf_files[file_index];
        if (udfread_file_seek(file, source_pos - file_start, SEEK_SET) != source_pos - file_start)
            return AVERROR(EIO);
        n = udfread_file_read(file, buf, (size_t)wanted);
    } else {
        do { n = pread(ctx->fds[file_index], buf, (size_t)wanted, (off_t)(source_pos - file_start)); }
        while (n < 0 && errno == EINTR && !is_cancelled(ctx->cancel));
    }
    if (n <= 0) return is_cancelled(ctx->cancel) ? AVERROR_EXIT : AVERROR(EIO);
    ctx->pos += n;
    return (int)n;
}

static int64_t source_seek(void *opaque, int64_t offset, int whence)
{
    SourceContext *ctx = (SourceContext *)opaque;
    if (whence == AVSEEK_SIZE) return ctx->stream_size;
    int base_whence = whence & ~AVSEEK_FORCE;
    int64_t next;
    switch (base_whence) {
        case SEEK_SET: next = offset; break;
        case SEEK_CUR: next = ctx->pos + offset; break;
        case SEEK_END: next = ctx->stream_size + offset; break;
        default: return AVERROR(EINVAL);
    }
    if (next < 0 || next > ctx->stream_size) return AVERROR(EINVAL);
    ctx->pos = next;
    return next;
}

#if LIBAVFORMAT_VERSION_MAJOR >= 61
static int output_write(void *opaque, const uint8_t *buf, int buf_size)
#else
static int output_write(void *opaque, uint8_t *buf, int buf_size)
#endif
{
    OutputContext *ctx = (OutputContext *)opaque;
    if (is_cancelled(ctx->cancel)) return AVERROR_EXIT;
    int written = 0;
    while (written < buf_size) {
        ssize_t n = pwrite(ctx->fd, buf + written, (size_t)(buf_size - written), (off_t)(ctx->pos + written));
        if (n < 0 && errno == EINTR) continue;
        if (n < 0) return AVERROR(errno);
        if (n == 0) return AVERROR(EIO);
        written += (int)n;
    }
    ctx->pos += written;
    return written;
}

static int64_t output_seek(void *opaque, int64_t offset, int whence)
{
    OutputContext *ctx = (OutputContext *)opaque;
    if (whence == AVSEEK_SIZE) {
        struct stat st;
        return fstat(ctx->fd, &st) == 0 ? st.st_size : AVERROR(errno);
    }
    int base_whence = whence & ~AVSEEK_FORCE;
    int64_t next;
    switch (base_whence) {
        case SEEK_SET: next = offset; break;
        case SEEK_CUR: next = ctx->pos + offset; break;
        case SEEK_END: {
            struct stat st;
            if (fstat(ctx->fd, &st) != 0) return AVERROR(errno);
            next = st.st_size + offset;
            break;
        }
        default: return AVERROR(EINVAL);
    }
    if (next < 0) return AVERROR(EINVAL);
    ctx->pos = next;
    return next;
}

static int init_source(JNIEnv *env, jintArray fd_array, jlongArray starts_array, jlongArray ends_array,
                       SourceContext *ctx, DvdUdfSource *udf, int title_set, CancelContext *cancel, char *error, size_t error_size)
{
    memset(ctx, 0, sizeof(*ctx));
    ctx->cancel = cancel;
    ctx->udf = udf;
    jsize fd_count = (*env)->GetArrayLength(env, fd_array);
    if (udf) {
        udf->cancelled = is_cancelled;
        udf->cancel_opaque = cancel;
        fd_count = 0;
        int gap = 0;
        for (int part = 1; part <= 9; ++part) {
            UDFFILE *file = dvd_udf_file(udf, title_set, part, 0);
            if (!file) { gap = 1; continue; }
            if (gap) {
                udfread_file_close(file);
                snprintf(error, error_size, "ISO title has a missing VOB part");
                return AVERROR_INVALIDDATA;
            }
            ctx->udf_files[fd_count++] = file;
        }
    }
    jsize span_count = (*env)->GetArrayLength(env, starts_array);
    if (fd_count <= 0 || span_count <= 0 || (*env)->GetArrayLength(env, ends_array) != span_count) {
        snprintf(error, error_size, "Invalid native DVD input arrays");
        return AVERROR(EINVAL);
    }

    ctx->fds = av_malloc_array((size_t)fd_count, sizeof(*ctx->fds));
    ctx->file_starts = av_malloc_array((size_t)fd_count, sizeof(*ctx->file_starts));
    ctx->spans = av_malloc_array((size_t)span_count, sizeof(*ctx->spans));
    if (!ctx->fds || !ctx->file_starts || !ctx->spans) return AVERROR(ENOMEM);
    ctx->fd_count = fd_count;
    ctx->span_count = span_count;

    jint *fds = (*env)->GetIntArrayElements(env, fd_array, NULL);
    jlong *starts = (*env)->GetLongArrayElements(env, starts_array, NULL);
    jlong *ends = (*env)->GetLongArrayElements(env, ends_array, NULL);
    if ((!fds && !udf) || !starts || !ends) {
        if (fds) (*env)->ReleaseIntArrayElements(env, fd_array, fds, JNI_ABORT);
        if (starts) (*env)->ReleaseLongArrayElements(env, starts_array, starts, JNI_ABORT);
        if (ends) (*env)->ReleaseLongArrayElements(env, ends_array, ends, JNI_ABORT);
        return AVERROR(ENOMEM);
    }

    int ret = 0;
    int64_t source_total = 0;
    for (int i = 0; i < fd_count; ++i) {
        struct stat st;
        ctx->fds[i] = udf ? -1 : fds[i];
        ctx->file_starts[i] = source_total;
        int64_t size = udf ? udfread_file_size(ctx->udf_files[i]) :
            (fstat(fds[i], &st) == 0 ? st.st_size : -1);
        unsigned char probe;
        if (size <= 0 || size % DVD_SECTOR_SIZE || size > INT64_MAX - source_total ||
            (!udf && pread(fds[i], &probe, 1, 0) != 1)) {
            snprintf(error, error_size, "DVD VOB descriptor %d is not seekable", i + 1);
            ret = AVERROR(EIO);
            goto done;
        }
        source_total += size;
    }
    ctx->total_source_size = source_total;

    int64_t stream_total = 0;
    for (int i = 0; i < span_count; ++i) {
        if (starts[i] < 0 || ends[i] <= starts[i] || ends[i] > source_total / DVD_SECTOR_SIZE) {
            snprintf(error, error_size, "DVD cell %d points outside title VOB data", i + 1);
            ret = AVERROR_INVALIDDATA;
            goto done;
        }
        int64_t start = starts[i] * DVD_SECTOR_SIZE;
        int64_t end = ends[i] * DVD_SECTOR_SIZE;
        if (start < 0 || end <= start || end > source_total || end - start > INT64_MAX - stream_total) {
            snprintf(error, error_size, "DVD cell %d points outside title VOB data", i + 1);
            ret = AVERROR_INVALIDDATA;
            goto done;
        }
        ctx->spans[i].source_start = start;
        ctx->spans[i].length = end - start;
        ctx->spans[i].stream_start = stream_total;
        stream_total += end - start;
    }
    ctx->stream_size = stream_total;

done:
    if (fds) (*env)->ReleaseIntArrayElements(env, fd_array, fds, JNI_ABORT);
    (*env)->ReleaseLongArrayElements(env, starts_array, starts, JNI_ABORT);
    (*env)->ReleaseLongArrayElements(env, ends_array, ends, JNI_ABORT);
    return ret;
}

static void free_source(SourceContext *ctx)
{
    for (int i = 0; i < 9; ++i) if (ctx->udf_files[i]) udfread_file_close(ctx->udf_files[i]);
    if (ctx->udf) { ctx->udf->cancelled = NULL; ctx->udf->cancel_opaque = NULL; }
    av_freep(&ctx->fds);
    av_freep(&ctx->file_starts);
    av_freep(&ctx->spans);
}

static int add_chapters(JNIEnv *env, AVFormatContext *out, jlongArray starts_array, jlongArray ends_array)
{
    jsize count = (*env)->GetArrayLength(env, starts_array);
    if (count <= 0 || (*env)->GetArrayLength(env, ends_array) != count) return AVERROR(EINVAL);
    jlong *starts = (*env)->GetLongArrayElements(env, starts_array, NULL);
    jlong *ends = (*env)->GetLongArrayElements(env, ends_array, NULL);
    if (!starts || !ends) {
        if (starts) (*env)->ReleaseLongArrayElements(env, starts_array, starts, JNI_ABORT);
        if (ends) (*env)->ReleaseLongArrayElements(env, ends_array, ends, JNI_ABORT);
        return AVERROR(ENOMEM);
    }

    AVChapter **chapters = av_calloc((size_t)count, sizeof(*chapters));
    if (!chapters) {
        (*env)->ReleaseLongArrayElements(env, starts_array, starts, JNI_ABORT);
        (*env)->ReleaseLongArrayElements(env, ends_array, ends, JNI_ABORT);
        return AVERROR(ENOMEM);
    }
    int ret = 0;
    for (int i = 0; i < count; ++i) {
        if (starts[i] < 0 || ends[i] <= starts[i]) { ret = AVERROR(EINVAL); break; }
        chapters[i] = av_mallocz(sizeof(*chapters[i]));
        if (!chapters[i]) { ret = AVERROR(ENOMEM); break; }
        chapters[i]->id = i;
        chapters[i]->time_base = (AVRational){1, 1000};
        chapters[i]->start = starts[i];
        chapters[i]->end = ends[i];
        char title[32];
        snprintf(title, sizeof(title), "Chapter %02d", i + 1);
        av_dict_set(&chapters[i]->metadata, "title", title, 0);
    }
    if (ret >= 0) {
        out->chapters = chapters;
        out->nb_chapters = count;
        chapters = NULL;
    }
    if (chapters) {
        for (int i = 0; i < count; ++i) {
            if (chapters[i]) {
                av_dict_free(&chapters[i]->metadata);
                av_free(chapters[i]);
            }
        }
        av_free(chapters);
    }
    (*env)->ReleaseLongArrayElements(env, starts_array, starts, JNI_ABORT);
    (*env)->ReleaseLongArrayElements(env, ends_array, ends, JNI_ABORT);
    return ret;
}

static void report_progress(JNIEnv *env, jobject thiz, jmethodID method, int percent)
{
    if (!method) return;
    (*env)->CallVoidMethod(env, thiz, method, percent);
    if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
}

JNIEXPORT jstring JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeVersionSummary(JNIEnv *env, jobject thiz)
{
    (void)thiz;
    char avformat[32], avcodec[32], avutil[32], summary[192];
    format_version(avformat, sizeof(avformat), avformat_version());
    format_version(avcodec, sizeof(avcodec), avcodec_version());
    format_version(avutil, sizeof(avutil), avutil_version());
    snprintf(summary, sizeof(summary), "FFmpeg %s (libavformat %s, libavcodec %s, libavutil %s)",
        av_version_info(), avformat, avcodec, avutil);
    return (*env)->NewStringUTF(env, summary);
}

static void throw_io(JNIEnv *env, const char *message)
{
    jclass cls = (*env)->FindClass(env, "java/io/IOException");
    if (cls) (*env)->ThrowNew(env, cls, message);
}

JNIEXPORT jlong JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeOpenIso(JNIEnv *env, jobject thiz, jint fd)
{
    (void)thiz;
    char error[256] = "Could not allocate UDF reader";
    DvdUdfSource *source = dvd_udf_open(fd, error, sizeof(error));
    if (!source) throw_io(env, error);
    return (jlong)(intptr_t)source;
}

JNIEXPORT void JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeCloseIso(JNIEnv *env, jobject thiz, jlong handle)
{
    (void)env; (void)thiz;
    dvd_udf_close((DvdUdfSource *)(intptr_t)handle);
}

JNIEXPORT jbyteArray JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeReadIsoIfo(JNIEnv *env, jobject thiz, jlong handle, jint title_set)
{
    (void)thiz;
    UDFFILE *file = dvd_udf_file((DvdUdfSource *)(intptr_t)handle, title_set, 0, 1);
    if (!file) return NULL;
    int64_t size = udfread_file_size(file);
    jbyteArray result = NULL;
    uint8_t *bytes = NULL;
    if (size <= 0 || size > 64 * 1024 * 1024) { throw_io(env, "Invalid ISO IFO size"); goto done; }
    bytes = malloc((size_t)size);
    if (!bytes) { throw_io(env, "Could not allocate IFO buffer"); goto done; }
    int64_t done_bytes = 0;
    while (done_bytes < size) {
        ssize_t n = udfread_file_read(file, bytes + done_bytes, (size_t)(size - done_bytes));
        if (n <= 0) { throw_io(env, "Truncated or unreadable IFO in ISO"); goto done; }
        done_bytes += n;
    }
    result = (*env)->NewByteArray(env, (jsize)size);
    if (result) (*env)->SetByteArrayRegion(env, result, 0, (jsize)size, (jbyte *)bytes);
done:
    free(bytes);
    udfread_file_close(file);
    return result;
}

static const char *track_type_name(enum AVMediaType type)
{
    switch (type) {
        case AVMEDIA_TYPE_VIDEO: return "video";
        case AVMEDIA_TYPE_AUDIO: return "audio";
        case AVMEDIA_TYPE_SUBTITLE: return "subtitle";
        default: return "other";
    }
}

static void metadata_field(const AVDictionary *metadata, const char *key, char *out, size_t out_size)
{
    AVDictionaryEntry *entry = av_dict_get(metadata, key, NULL, 0);
    const char *source = (entry && entry->value && entry->value[0]) ? entry->value : "-";
    size_t j = 0;
    for (size_t i = 0; source[i] && j + 1 < out_size; ++i) {
        unsigned char c = (unsigned char)source[i];
        out[j++] = (c == '\t' || c == '\r' || c == '\n') ? ' ' : (char)c;
    }
    out[j] = '\0';
}

JNIEXPORT jobjectArray JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeProbeTracks(
    JNIEnv *env, jobject thiz, jintArray fd_array, jlongArray starts_array, jlongArray ends_array,
    jlong iso_handle, jint title_set)
{
    jclass engine_class = (*env)->GetObjectClass(env, thiz);
    CancelContext cancel = {env, thiz, (*env)->GetMethodID(env, engine_class, "isNativeCancelled", "()Z")};
    if (!cancel.method) return NULL;
    char error[512] = {0};
    int ret = 0;
    SourceContext source;
    AVIOContext *input_io = NULL;
    AVFormatContext *input = NULL;
    jobjectArray result = NULL;

    ret = init_source(env, fd_array, starts_array, ends_array, &source,
        (DvdUdfSource *)(intptr_t)iso_handle, title_set, &cancel, error, sizeof(error));
    if (ret < 0) goto cleanup_probe;

    uint8_t *input_buffer = av_malloc(IO_BUFFER_SIZE);
    if (!input_buffer) { ret = AVERROR(ENOMEM); goto cleanup_probe; }
    input_io = avio_alloc_context(input_buffer, IO_BUFFER_SIZE, 0, &source, source_read, NULL, source_seek);
    if (!input_io) { av_free(input_buffer); ret = AVERROR(ENOMEM); goto cleanup_probe; }
    input = avformat_alloc_context();
    if (!input) { ret = AVERROR(ENOMEM); goto cleanup_probe; }
    input->pb = input_io;
    input->probesize = 100000000;
    input->max_analyze_duration = 100000000;
    input->interrupt_callback = (AVIOInterruptCB){is_cancelled, &cancel};
    input->flags |= AVFMT_FLAG_CUSTOM_IO;
    ret = avformat_open_input(&input, NULL, NULL, NULL);
    if (ret < 0) { ff_error(error, sizeof(error), "Could not open selected DVD program stream", ret); goto cleanup_probe; }
    ret = avformat_find_stream_info(input, NULL);
    if (ret < 0) { ff_error(error, sizeof(error), "Could not probe DVD streams", ret); goto cleanup_probe; }

    int count = 0;
    for (unsigned i = 0; i < input->nb_streams; ++i) {
        enum AVMediaType type = input->streams[i]->codecpar->codec_type;
        if (type == AVMEDIA_TYPE_VIDEO || type == AVMEDIA_TYPE_AUDIO || type == AVMEDIA_TYPE_SUBTITLE) count++;
    }
    jclass string_class = (*env)->FindClass(env, "java/lang/String");
    if (!string_class) { ret = AVERROR_EXTERNAL; goto cleanup_probe; }
    result = (*env)->NewObjectArray(env, count, string_class, NULL);
    if (!result) { ret = AVERROR(ENOMEM); goto cleanup_probe; }

    int row = 0;
    for (unsigned i = 0; i < input->nb_streams; ++i) {
        AVStream *stream = input->streams[i];
        AVCodecParameters *par = stream->codecpar;
        enum AVMediaType type = par->codec_type;
        if (type != AVMEDIA_TYPE_VIDEO && type != AVMEDIA_TYPE_AUDIO && type != AVMEDIA_TYPE_SUBTITLE) continue;
        char language[128], title[256], layout[128] = "-", record[1024];
        metadata_field(stream->metadata, "language", language, sizeof(language));
        metadata_field(stream->metadata, "title", title, sizeof(title));
        int channels = 0;
        if (type == AVMEDIA_TYPE_AUDIO) {
            channels = par->ch_layout.nb_channels;
            if (channels > 0 && av_channel_layout_describe(&par->ch_layout, layout, sizeof(layout)) < 0) strcpy(layout, "-");
        }
        snprintf(record, sizeof(record), "%u\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s",
            i, track_type_name(type), avcodec_get_name(par->codec_id), language, title,
            par->width, par->height, channels, layout);
        jstring value = (*env)->NewStringUTF(env, record);
        if (!value) { ret = AVERROR(ENOMEM); goto cleanup_probe; }
        (*env)->SetObjectArrayElement(env, result, row++, value);
        (*env)->DeleteLocalRef(env, value);
        if ((*env)->ExceptionCheck(env)) { ret = AVERROR_EXTERNAL; goto cleanup_probe; }
    }

cleanup_probe:
    if (input) {
        if (input->iformat) avformat_close_input(&input);
        else avformat_free_context(input);
    }
    if (input_io) { av_freep(&input_io->buffer); avio_context_free(&input_io); }
    free_source(&source);
    if (ret < 0) {
        if (!(*env)->ExceptionCheck(env)) {
            if (error[0] == '\0') ff_error(error, sizeof(error), "Native metadata probe failed", ret);
            throw_io(env, error);
        }
        return NULL;
    }
    return result;
}

JNIEXPORT jstring JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeRemux(
    JNIEnv *env, jobject thiz, jintArray fd_array, jlongArray starts_array, jlongArray ends_array,
    jint output_fd, jlongArray chapter_starts, jlongArray chapter_ends, jintArray selected_streams, jlong iso_handle, jint title_set)
{
    jclass engine_class = (*env)->GetObjectClass(env, thiz);
    CancelContext cancel = {env, thiz, (*env)->GetMethodID(env, engine_class, "isNativeCancelled", "()Z")};
    if (!cancel.method) return NULL; /* pending JNI exception propagates */
    char error[512] = {0};
    int ret = 0;
    SourceContext source;
    OutputContext output = { .fd = output_fd, .cancel = &cancel, .pos = 0 };
    AVIOContext *input_io = NULL, *output_io = NULL;
    AVFormatContext *input = NULL, *out = NULL;
    AVPacket *packet = NULL;
    int *stream_map = NULL;
    jint *selected_indexes = NULL;
    jsize selected_count = -1;
    int64_t timestamp_origin_us = 0;


    ret = init_source(env, fd_array, starts_array, ends_array, &source, (DvdUdfSource *)(intptr_t)iso_handle, title_set, &cancel, error, sizeof(error));
    if (ret < 0) goto cleanup;
    if (is_cancelled(&cancel)) { ret = AVERROR_EXIT; goto cleanup; }
    if (ftruncate(output_fd, 0) != 0) {
        snprintf(error, sizeof(error), "Could not truncate output document: %s", strerror(errno));
        ret = AVERROR(errno); goto cleanup;
    }

    uint8_t *input_buffer = av_malloc(IO_BUFFER_SIZE);
    if (!input_buffer) { ret = AVERROR(ENOMEM); goto cleanup; }
    input_io = avio_alloc_context(input_buffer, IO_BUFFER_SIZE, 0, &source, source_read, NULL, source_seek);
    if (!input_io) { av_free(input_buffer); ret = AVERROR(ENOMEM); goto cleanup; }

    input = avformat_alloc_context();
    if (!input) { ret = AVERROR(ENOMEM); goto cleanup; }
    input->pb = input_io;
    input->probesize = 100000000;
    input->max_analyze_duration = 100000000;
    input->interrupt_callback = (AVIOInterruptCB){is_cancelled, &cancel};
    input->flags |= AVFMT_FLAG_CUSTOM_IO;
    ret = avformat_open_input(&input, NULL, NULL, NULL);
    if (ret < 0) { ff_error(error, sizeof(error), "Could not open selected DVD program stream", ret); goto cleanup; }
    ret = avformat_find_stream_info(input, NULL);
    if (ret < 0) { ff_error(error, sizeof(error), "Could not probe DVD streams", ret); goto cleanup; }

    /* One clock origin for every stream preserves A/V offsets and aligns chapters.
       Per-stream zeroing would silently erase synchronization differences. */
    if (input->start_time != AV_NOPTS_VALUE) timestamp_origin_us = input->start_time;

    ret = avformat_alloc_output_context2(&out, NULL, "matroska", NULL);
    if (ret < 0 || !out) { if (ret >= 0) ret = AVERROR_UNKNOWN; ff_error(error, sizeof(error), "Could not create Matroska muxer", ret); goto cleanup; }

    out->avoid_negative_ts = AVFMT_AVOID_NEG_TS_DISABLED;

    stream_map = av_malloc_array(input->nb_streams, sizeof(*stream_map));
    if (!stream_map) { ret = AVERROR(ENOMEM); goto cleanup; }
    for (unsigned i = 0; i < input->nb_streams; ++i) stream_map[i] = -1;

    if (selected_streams) {
        selected_count = (*env)->GetArrayLength(env, selected_streams);
        if (selected_count <= 0) { snprintf(error, sizeof(error), "No streams selected"); ret = AVERROR(EINVAL); goto cleanup; }
        selected_indexes = (*env)->GetIntArrayElements(env, selected_streams, NULL);
        if (!selected_indexes) { ret = AVERROR(ENOMEM); goto cleanup; }
    }

    for (unsigned i = 0; i < input->nb_streams; ++i) {
        AVStream *in_stream = input->streams[i];
        enum AVMediaType type = in_stream->codecpar->codec_type;
        if (type != AVMEDIA_TYPE_VIDEO && type != AVMEDIA_TYPE_AUDIO && type != AVMEDIA_TYPE_SUBTITLE) continue;
        if (selected_indexes) {
            int wanted = 0;
            for (jsize j = 0; j < selected_count; ++j) if (selected_indexes[j] == (jint)i) { wanted = 1; break; }
            if (!wanted) continue;
        }
        AVStream *out_stream = avformat_new_stream(out, NULL);
        if (!out_stream) { ret = AVERROR(ENOMEM); goto cleanup; }
        stream_map[i] = out_stream->index;
        ret = avcodec_parameters_copy(out_stream->codecpar, in_stream->codecpar);
        if (ret < 0) { ff_error(error, sizeof(error), "Could not copy stream parameters", ret); goto cleanup; }
        out_stream->codecpar->codec_tag = 0;
        out_stream->time_base = in_stream->time_base;
        av_dict_copy(&out_stream->metadata, in_stream->metadata, 0);
    }
    if (out->nb_streams == 0) { snprintf(error, sizeof(error), "DVD title contains no Matroska-compatible streams"); ret = AVERROR_STREAM_NOT_FOUND; goto cleanup; }

    ret = add_chapters(env, out, chapter_starts, chapter_ends);
    if (ret < 0) { ff_error(error, sizeof(error), "Could not add DVD chapters", ret); goto cleanup; }
    av_dict_set(&out->metadata, "encoder", "MattMux Android (stream copy)", 0);

    uint8_t *output_buffer = av_malloc(IO_BUFFER_SIZE);
    if (!output_buffer) { ret = AVERROR(ENOMEM); goto cleanup; }
    output_io = avio_alloc_context(output_buffer, IO_BUFFER_SIZE, 1, &output, NULL, output_write, output_seek);
    if (!output_io) { av_free(output_buffer); ret = AVERROR(ENOMEM); goto cleanup; }
    out->pb = output_io;
    out->flags |= AVFMT_FLAG_CUSTOM_IO;

    ret = avformat_write_header(out, NULL);
    if (ret < 0) { ff_error(error, sizeof(error), "Could not write Matroska header", ret); goto cleanup; }


    packet = av_packet_alloc();
    if (!packet) { ret = AVERROR(ENOMEM); goto cleanup; }
    jclass cls = (*env)->GetObjectClass(env, thiz);
    jmethodID progress_method = cls ? (*env)->GetMethodID(env, cls, "onNativeProgress", "(I)V") : NULL;
    if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); progress_method = NULL; }
    int last_percent = -1;

    while ((ret = av_read_frame(input, packet)) >= 0) {
        if (is_cancelled(&cancel)) { ret = AVERROR_EXIT; break; }
        int in_index = packet->stream_index;
        int out_index = (in_index >= 0 && (unsigned)in_index < input->nb_streams) ? stream_map[in_index] : -1;
        if (out_index >= 0) {
            AVStream *in_stream = input->streams[in_index];
            AVStream *out_stream = out->streams[out_index];
            int64_t origin = av_rescale_q(timestamp_origin_us, AV_TIME_BASE_Q, in_stream->time_base);
            if (packet->pts != AV_NOPTS_VALUE) packet->pts -= origin;
            if (packet->dts != AV_NOPTS_VALUE) packet->dts -= origin;
            packet->stream_index = out_index;
            av_packet_rescale_ts(packet, in_stream->time_base, out_stream->time_base);
            packet->pos = -1;
            ret = av_interleaved_write_frame(out, packet);
            if (ret < 0) { ff_error(error, sizeof(error), "Matroska write failed", ret); av_packet_unref(packet); break; }
        }
        av_packet_unref(packet);
        int percent = source.stream_size > 0 ? (int)((source.pos * 100) / source.stream_size) : 0;
        if (percent > 99) percent = 99;
        if (percent < last_percent) percent = last_percent;
        if (percent != last_percent) { report_progress(env, thiz, progress_method, percent); last_percent = percent; }
    }
    if (ret == AVERROR_EOF) {
        ret = (input_io->error < 0 && input_io->error != AVERROR_EOF) ? input_io->error : 0;
    }
    if (ret == AVERROR_EXIT || is_cancelled(&cancel)) {
        snprintf(error, sizeof(error), "Remux cancelled");
        ret = AVERROR_EXIT;
    } else if (ret < 0 && error[0] == '\0') {
        ff_error(error, sizeof(error), "DVD read failed", ret);
    }

    if (ret >= 0) {
        ret = av_write_trailer(out);
        if (ret < 0) ff_error(error, sizeof(error), "Could not finalize Matroska file", ret);
        else {
            avio_flush(output_io);
            if (output_io->error < 0) ret = output_io->error;
            else if (fsync(output_fd) < 0) ret = AVERROR(errno);
            if (is_cancelled(&cancel)) ret = AVERROR_EXIT;
            if (ret < 0) ff_error(error, sizeof(error), "Could not flush output document", ret);
            else report_progress(env, thiz, progress_method, 100);
        }
    }

cleanup:
    if (packet) av_packet_free(&packet);
    if (selected_indexes) (*env)->ReleaseIntArrayElements(env, selected_streams, selected_indexes, JNI_ABORT);
    av_freep(&stream_map);
    if (out) {
        out->pb = NULL;
        avformat_free_context(out);
    }
    if (output_io) { av_freep(&output_io->buffer); avio_context_free(&output_io); }
    if (input) {
        if (input->iformat) avformat_close_input(&input);
        else avformat_free_context(input);
    }
    if (input_io) { av_freep(&input_io->buffer); avio_context_free(&input_io); }
    free_source(&source);

    if (ret >= 0) return NULL;
    if (ret == AVERROR_EXIT) snprintf(error, sizeof(error), "Remux cancelled");
    if (error[0] == '\0') ff_error(error, sizeof(error), "Native remux failed", ret);
    return (*env)->NewStringUTF(env, error);
}
