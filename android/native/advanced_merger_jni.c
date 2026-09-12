/* Included by mattmux_jni.c to share JNI cancellation and metadata helpers. */
typedef struct {
    AVFormatContext *format;
    AVPacket *packet;
    int *map;
    int64_t *next_ts;
    int eof;
} MergerInput;

static int merger_open(const char *path, AVFormatContext **input, CancelContext *cancel)
{
    *input = avformat_alloc_context();
    if (!*input) return AVERROR(ENOMEM);
    (*input)->interrupt_callback = (AVIOInterruptCB){is_cancelled, cancel};
    (*input)->flags |= AVFMT_FLAG_GENPTS;
    int ret = avformat_open_input(input, path, NULL, NULL);
    if (ret >= 0) ret = avformat_find_stream_info(*input, NULL);
    return ret;
}

static int merger_chapters(const char *path, AVFormatContext **input, CancelContext *cancel)
{
    char header[32];
    FILE *file = fopen(path, "rb");
    if (!file) return AVERROR(errno);
    int valid = fgets(header, sizeof(header), file) &&
        (!strcmp(header, ";FFMETADATA1\n") || !strcmp(header, ";FFMETADATA1\r\n"));
    fclose(file);
    if (!valid) return AVERROR_INVALIDDATA;
    *input = avformat_alloc_context();
    if (!*input) return AVERROR(ENOMEM);
    (*input)->interrupt_callback = (AVIOInterruptCB){is_cancelled, cancel};
    int ret = avformat_open_input(input, path, av_find_input_format("ffmetadata"), NULL);
    if (ret < 0) return ret;
    if (!(*input)->nb_chapters) return AVERROR_INVALIDDATA;
    for (unsigned i = 0; i < (*input)->nb_chapters; ++i) {
        AVChapter *chapter = (*input)->chapters[i];
        if (chapter->time_base.num <= 0 || chapter->time_base.den <= 0 || chapter->start < 0 || chapter->end <= chapter->start)
            return AVERROR_INVALIDDATA;
    }
    return 0;
}

static int merger_next(MergerInput *input, CancelContext *cancel)
{
    av_packet_unref(input->packet);
    for (;;) {
        if (is_cancelled(cancel)) return AVERROR_EXIT;
        int ret = av_read_frame(input->format, input->packet);
        if (ret == AVERROR_EOF) { input->eof = 1; return 0; }
        if (ret < 0) return ret;
        AVPacket *packet = input->packet;
        int index = packet->stream_index;
        if (index < 0 || (unsigned)index >= input->format->nb_streams || input->map[index] < 0) {
            av_packet_unref(packet); continue;
        }
        AVStream *stream = input->format->streams[index];
        int64_t duration = packet->duration;
        if (duration <= 0 && stream->codecpar->codec_type == AVMEDIA_TYPE_VIDEO) {
            AVRational rate = av_guess_frame_rate(input->format, stream, NULL);
            if (rate.num > 0 && rate.den > 0) duration = av_rescale_q(1, av_inv_q(rate), stream->time_base);
        }
        if (duration <= 0 && stream->codecpar->codec_type == AVMEDIA_TYPE_AUDIO && stream->codecpar->sample_rate > 0) {
            int samples = av_get_audio_frame_duration2(stream->codecpar, packet->size);
            if (samples > 0) duration = av_rescale_q(samples, (AVRational){1, stream->codecpar->sample_rate}, stream->time_base);
        }
        if (packet->dts == AV_NOPTS_VALUE && packet->pts == AV_NOPTS_VALUE) {
            if (duration <= 0 || stream->codecpar->video_delay > 0) return AVERROR_INVALIDDATA;
            packet->dts = packet->pts = input->next_ts[index];
        } else if (packet->dts == AV_NOPTS_VALUE) packet->dts = packet->pts;
        if (packet->pts == AV_NOPTS_VALUE) packet->pts = packet->dts;
        packet->duration = duration;
        input->next_ts[index] = packet->dts + duration;
        if (input->format->start_time != AV_NOPTS_VALUE) {
            int64_t origin = av_rescale_q(input->format->start_time, AV_TIME_BASE_Q, stream->time_base);
            packet->pts -= origin; packet->dts -= origin;
        }
        return 0;
    }
}

static int merger_mux(const char **paths, int count, const int *sources, const int *streams, int selected,
    const char *chapter_path, const char *destination, CancelContext *cancel, char *error, size_t error_size)
{
    int ret = 0;
    MergerInput *inputs = av_calloc(count, sizeof(*inputs));
    AVFormatContext *output = NULL, *chapters = NULL;
    if (!inputs) return AVERROR(ENOMEM);
    if (count <= 0 || count > 1024 || selected <= 0) { ret = AVERROR(EINVAL); goto cleanup_merger; }
    for (int i = 0; i < count; ++i) {
        if ((ret = merger_open(paths[i], &inputs[i].format, cancel)) < 0) goto cleanup_merger;
        inputs[i].map = av_malloc_array(inputs[i].format->nb_streams, sizeof(int));
        inputs[i].next_ts = av_calloc(inputs[i].format->nb_streams, sizeof(int64_t));
        inputs[i].packet = av_packet_alloc();
        if (!inputs[i].map || !inputs[i].next_ts || !inputs[i].packet) { ret = AVERROR(ENOMEM); goto cleanup_merger; }
        for (unsigned j = 0; j < inputs[i].format->nb_streams; ++j) inputs[i].map[j] = -1;
    }
    ret = avformat_alloc_output_context2(&output, NULL, "matroska", destination);
    if (ret < 0 || !output) { ret = AVERROR(ENOMEM); goto cleanup_merger; }
    output->interrupt_callback = (AVIOInterruptCB){is_cancelled, cancel};
    for (int i = 0; i < selected; ++i) {
        int source = sources[i], index = streams[i];
        if (source < 0 || source >= count || index < 0 || (unsigned)index >= inputs[source].format->nb_streams) { ret = AVERROR(EINVAL); goto cleanup_merger; }
        if (inputs[source].map[index] >= 0) continue;
        AVStream *in = inputs[source].format->streams[index];
        if (in->codecpar->codec_type != AVMEDIA_TYPE_VIDEO && in->codecpar->codec_type != AVMEDIA_TYPE_AUDIO && in->codecpar->codec_type != AVMEDIA_TYPE_SUBTITLE) { ret = AVERROR(EINVAL); goto cleanup_merger; }
        AVStream *out = avformat_new_stream(output, NULL);
        if (!out) { ret = AVERROR(ENOMEM); goto cleanup_merger; }
        if ((ret = avcodec_parameters_copy(out->codecpar, in->codecpar)) < 0) goto cleanup_merger;
        out->codecpar->codec_tag = 0; out->time_base = in->time_base; out->disposition = in->disposition;
        av_dict_copy(&out->metadata, in->metadata, 0);
        inputs[source].map[index] = out->index;
    }
    if (chapter_path) {
        if ((ret = merger_chapters(chapter_path, &chapters, cancel)) < 0) goto cleanup_merger;
        output->chapters = av_calloc(chapters->nb_chapters, sizeof(*output->chapters));
        if (!output->chapters) { ret = AVERROR(ENOMEM); goto cleanup_merger; }
        for (unsigned i = 0; i < chapters->nb_chapters; ++i) {
            AVChapter *in = chapters->chapters[i], *out = av_mallocz(sizeof(*out));
            if (!out) { ret = AVERROR(ENOMEM); goto cleanup_merger; }
            out->id = in->id; out->time_base = in->time_base; out->start = in->start; out->end = in->end;
            av_dict_copy(&out->metadata, in->metadata, 0); output->chapters[output->nb_chapters++] = out;
        }
    }
    if ((ret = avio_open(&output->pb, destination, AVIO_FLAG_WRITE)) < 0) goto cleanup_merger;
    if ((ret = avformat_write_header(output, NULL)) < 0) goto cleanup_merger;
    for (int i = 0; i < count; ++i) if ((ret = merger_next(&inputs[i], cancel)) < 0) goto cleanup_merger;
    for (;;) {
        int next = -1;
        for (int i = 0; i < count; ++i) if (!inputs[i].eof) {
            if (next < 0 || av_compare_ts(inputs[i].packet->dts, inputs[i].format->streams[inputs[i].packet->stream_index]->time_base,
                inputs[next].packet->dts, inputs[next].format->streams[inputs[next].packet->stream_index]->time_base) < 0) next = i;
        }
        if (next < 0) break;
        AVPacket *packet = inputs[next].packet;
        AVStream *in = inputs[next].format->streams[packet->stream_index];
        int mapped = inputs[next].map[packet->stream_index];
        av_packet_rescale_ts(packet, in->time_base, output->streams[mapped]->time_base);
        packet->stream_index = mapped; packet->pos = -1;
        if ((ret = av_interleaved_write_frame(output, packet)) < 0) goto cleanup_merger;
        if ((ret = merger_next(&inputs[next], cancel)) < 0) goto cleanup_merger;
    }
    ret = av_write_trailer(output);
    if (ret >= 0) { avio_flush(output->pb); if (output->pb->error < 0) ret = output->pb->error; }
cleanup_merger:
    if (ret < 0) ff_error(error, error_size, "Advanced merge failed (check selected codecs, raw timestamps, chapter format, and free space)", ret);
    if (output) { int close_ret = output->pb ? avio_closep(&output->pb) : 0; if (ret >= 0 && close_ret < 0) { ret = close_ret; ff_error(error, error_size, "Closing output failed", ret); } avformat_free_context(output); }
    avformat_close_input(&chapters);
    for (int i = 0; i < count; ++i) { av_packet_free(&inputs[i].packet); av_free(inputs[i].map); av_free(inputs[i].next_ts); avformat_close_input(&inputs[i].format); }
    av_free(inputs);
    return ret;
}

JNIEXPORT jobjectArray JNICALL
Java_io_github_maas3n_mattmux_AdvancedMergerNative_probe(JNIEnv *env, jobject thiz, jstring filename)
{
    CancelContext cancel = {env, thiz, (*env)->GetMethodID(env, (*env)->GetObjectClass(env, thiz), "isNativeCancelled", "()Z")};
    const char *path = (*env)->GetStringUTFChars(env, filename, NULL);
    if (!path) return NULL;
    AVFormatContext *input = NULL;
    int ret = merger_open(path, &input, &cancel);
    (*env)->ReleaseStringUTFChars(env, filename, path);
    jobjectArray result = NULL;
    if (ret >= 0) {
        int count = 0;
        for (unsigned i = 0; i < input->nb_streams; ++i) {
            enum AVMediaType type = input->streams[i]->codecpar->codec_type;
            if (type == AVMEDIA_TYPE_VIDEO || type == AVMEDIA_TYPE_AUDIO || type == AVMEDIA_TYPE_SUBTITLE) ++count;
        }
        result = (*env)->NewObjectArray(env, count, (*env)->FindClass(env, "java/lang/String"), NULL);
        int row = 0;
        for (unsigned i = 0; result && i < input->nb_streams; ++i) {
            AVStream *stream = input->streams[i]; AVCodecParameters *p = stream->codecpar;
            if (p->codec_type != AVMEDIA_TYPE_VIDEO && p->codec_type != AVMEDIA_TYPE_AUDIO && p->codec_type != AVMEDIA_TYPE_SUBTITLE) continue;
            char language[128], title[256], layout[128] = "-", record[1024];
            metadata_field(stream->metadata, "language", language, sizeof(language)); metadata_field(stream->metadata, "title", title, sizeof(title));
            if (p->ch_layout.nb_channels > 0) av_channel_layout_describe(&p->ch_layout, layout, sizeof(layout));
            snprintf(record, sizeof(record), "%u\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s", i, track_type_name(p->codec_type), avcodec_get_name(p->codec_id), language, title, p->width, p->height, p->ch_layout.nb_channels, layout);
            jstring value = (*env)->NewStringUTF(env, record);
            if (!value) { ret = AVERROR(ENOMEM); break; }
            (*env)->SetObjectArrayElement(env, result, row++, value); (*env)->DeleteLocalRef(env, value);
        }
    }
    avformat_close_input(&input);
    if (ret < 0) { char error[512]; ff_error(error, sizeof(error), "Cannot probe media", ret); if (!(*env)->ExceptionCheck(env)) throw_io(env, error); return NULL; }
    return result;
}

JNIEXPORT jstring JNICALL
Java_io_github_maas3n_mattmux_AdvancedMergerNative_validateChapters(JNIEnv *env, jobject thiz, jstring filename)
{
    CancelContext cancel = {env, thiz, (*env)->GetMethodID(env, (*env)->GetObjectClass(env, thiz), "isNativeCancelled", "()Z")};
    const char *path = (*env)->GetStringUTFChars(env, filename, NULL); if (!path) return NULL;
    AVFormatContext *input = NULL; int ret = merger_chapters(path, &input, &cancel); avformat_close_input(&input);
    (*env)->ReleaseStringUTFChars(env, filename, path);
    return ret < 0 ? (*env)->NewStringUTF(env, "Choose a valid FFMETADATA1 file containing chapters with valid start/end times") : NULL;
}

JNIEXPORT jstring JNICALL
Java_io_github_maas3n_mattmux_AdvancedMergerNative_mux(JNIEnv *env, jobject thiz, jobjectArray filenames, jintArray source_array, jintArray stream_array, jstring chapter_file, jstring output_file)
{
    CancelContext cancel = {env, thiz, (*env)->GetMethodID(env, (*env)->GetObjectClass(env, thiz), "isNativeCancelled", "()Z")};
    int count = (*env)->GetArrayLength(env, filenames), selected = (*env)->GetArrayLength(env, source_array);
    if (count < 1 || count > 1024 || selected < 1 || selected != (*env)->GetArrayLength(env, stream_array)) return (*env)->NewStringUTF(env, "Invalid stream selection");
    const char **paths = av_calloc(count, sizeof(char *));
    jstring *refs = av_calloc(count, sizeof(jstring));
    jint *sources = NULL, *streams = NULL; const char *chapters = NULL, *output = NULL;
    int ret = AVERROR(ENOMEM); char error[512] = "Cannot allocate merger inputs";
    if (!paths || !refs) goto cleanup_jni_merger;
    for (int i = 0; i < count; ++i) { refs[i] = (*env)->GetObjectArrayElement(env, filenames, i); if (!refs[i]) goto cleanup_jni_merger; paths[i] = (*env)->GetStringUTFChars(env, refs[i], NULL); if (!paths[i]) goto cleanup_jni_merger; }
    sources = (*env)->GetIntArrayElements(env, source_array, NULL); streams = (*env)->GetIntArrayElements(env, stream_array, NULL);
    if (chapter_file) chapters = (*env)->GetStringUTFChars(env, chapter_file, NULL);
    output = (*env)->GetStringUTFChars(env, output_file, NULL);
    if (!sources || !streams || !output || (chapter_file && !chapters)) goto cleanup_jni_merger;
    ret = merger_mux(paths, count, sources, streams, selected, chapters, output, &cancel, error, sizeof(error));
cleanup_jni_merger:
    if (sources) (*env)->ReleaseIntArrayElements(env, source_array, sources, JNI_ABORT);
    if (streams) (*env)->ReleaseIntArrayElements(env, stream_array, streams, JNI_ABORT);
    if (chapters) (*env)->ReleaseStringUTFChars(env, chapter_file, chapters);
    if (output) (*env)->ReleaseStringUTFChars(env, output_file, output);
    for (int i = 0; i < count; ++i) { if (paths && paths[i]) (*env)->ReleaseStringUTFChars(env, refs[i], paths[i]); if (refs && refs[i]) (*env)->DeleteLocalRef(env, refs[i]); }
    av_free(paths); av_free(refs);
    return ret < 0 && !(*env)->ExceptionCheck(env) ? (*env)->NewStringUTF(env, error) : NULL;
}
