/* Shares the production demuxer, cancellation, and JNI helpers with the merger. */
#include <libavcodec/bsf.h>

typedef struct { AVFormatContext *format; AVBSFContext *bsf; char path[4096]; int count; } DemuxOutput;
static const char *demux_format(enum AVCodecID codec, int vob, const char **extension, const char **filter)
{
    *filter = NULL;
#define F(id, ext, mux) case AV_CODEC_ID_##id: *extension = ext; return mux
    switch (codec) {
        case AV_CODEC_ID_H264: *extension = "h264"; *filter = "h264_mp4toannexb"; return "h264";
        case AV_CODEC_ID_HEVC: *extension = "h265"; *filter = "hevc_mp4toannexb"; return "hevc";
        case AV_CODEC_ID_MPEG2VIDEO: *extension = vob ? "VOB" : "mpeg2"; return vob ? "vob" : "mpeg2video";
        F(MPEG1VIDEO, "mpeg1", "mpeg1video"); F(MPEG4, "m4v", "m4v");
        F(AC3, "ac3", "ac3"); F(EAC3, "eac3", "eac3"); F(DTS, "dts", "dts");
        F(AAC, "aac", "adts"); F(MP2, "mp2", "mp2"); F(MP3, "mp3", "mp3");
        F(FLAC, "flac", "flac"); F(TRUEHD, "thd", "truehd");
        F(SUBRIP, "srt", "srt"); F(ASS, "ass", "ass"); F(SSA, "ssa", "ass");
        F(HDMV_PGS_SUBTITLE, "sup", "sup"); F(DVD_SUBTITLE, "sub", "vob");
        F(PCM_S16LE, "wav", "wav"); F(PCM_S24LE, "wav", "wav"); F(PCM_S32LE, "wav", "wav");
        default: return NULL;
    }
#undef F
}

static void demux_time(FILE *file, int64_t ms, char sep)
{
    fprintf(file, "%02lld:%02lld:%02lld%c%03lld", (long long)(ms/3600000), (long long)(ms/60000%60), (long long)(ms/1000%60), sep, (long long)(ms%1000));
}

static int demux_vobsub_index(const char *path, AVCodecParameters *params, const char *language)
{
    if (!params->extradata || !strstr((char *)params->extradata, "palette:") || !strstr((char *)params->extradata, "size:")) return AVERROR_INVALIDDATA;
    char idx_path[4096]; snprintf(idx_path, sizeof(idx_path), "%s", path);
    size_t len = strlen(idx_path); if (len < 4) return AVERROR(EINVAL);
    memcpy(idx_path+len-3, "idx", 3);
    FILE *input = fopen(path, "rb"), *idx = fopen(idx_path, "wb");
    if (!input || !idx) { if (input) fclose(input); if (idx) fclose(idx); return AVERROR(EIO); }
    fprintf(idx, "# VobSub index file, v7 (do not modify this line!)\n%.*s\nid: %s, index: 0\n", params->extradata_size, params->extradata, language && strlen(language)==2 ? language : "--");
    uint8_t b[2048]; int64_t position=0, last=-1; int count=0, ret=0; size_t n;
    while ((n=fread(b,1,sizeof(b),input))) {
        if (n!=sizeof(b) || memcmp(b,"\x00\x00\x01\xba",4)) { ret=AVERROR_INVALIDDATA; break; }
        for (size_t i=14; i+14<n; ++i) {
            if (memcmp(b+i,"\x00\x00\x01\xbd",4) || !(b[i+7]&0x80) || b[i+8]<5) continue;
            size_t payload=i+9+b[i+8]; if (payload>=n || b[payload]!=0x20) continue;
            uint8_t *p=b+i+9;
            int64_t pts=((int64_t)(p[0]&14)<<29)|((int64_t)p[1]<<22)|((int64_t)(p[2]&254)<<14)|((int64_t)p[3]<<7)|(p[4]>>1);
            if (pts!=last) { fputs("timestamp: ",idx); demux_time(idx,pts/90,':'); fprintf(idx,", filepos: %09llx\n",(unsigned long long)position); last=pts; ++count; }
            break;
        }
        position+=2048;
    }
    if (!count || ferror(input) || ferror(idx)) ret=AVERROR_INVALIDDATA;
    if (fclose(idx)) ret=AVERROR(EIO);
    fclose(input); return ret;
}

static int demux_write(DemuxOutput *out, AVPacket *packet, AVRational timebase)
{
    av_packet_rescale_ts(packet,timebase,out->format->streams[0]->time_base);
    packet->stream_index=0; packet->pos=-1;
    int ret=av_interleaved_write_frame(out->format,packet);
    if (ret>=0) ++out->count;
    return ret;
}

/* The output writer accepts either a regular file or a planned DVD packet reader.
 * The metadata context remains stable even when DVD clock segments reopen. */
typedef int (*DemuxRead)(void *, AVPacket *);
static int demux_file_read(void *opaque, AVPacket *packet) { return av_read_frame(opaque, packet); }

static int demux_input(AVFormatContext *input, DemuxRead read_packet, void *reader,
    int64_t input_size, const char *directory, const int *selected, int selected_count,
    int chapters, int vob, CancelContext *cancel, jmethodID progress)
{
    int last_percent = -1;
    int64_t processed = 0;
    DemuxOutput *outputs=NULL; AVPacket *packet=NULL, *filtered=NULL; int ret=0;
    report_progress(cancel->env, cancel->engine, progress, 0);
    outputs=av_calloc(input->nb_streams,sizeof(*outputs)); packet=av_packet_alloc(); filtered=av_packet_alloc();
    if (!outputs || !packet || !filtered) { ret=AVERROR(ENOMEM); goto done; }
    if (!selected_count) { ret=AVERROR(EINVAL); goto done; }
    for (int n=0;n<selected_count;++n) {
        int i=selected[n]; if (i<0 || (unsigned)i>=input->nb_streams || outputs[i].format) { ret=AVERROR(EINVAL); goto done; }
        AVStream *src=input->streams[i]; const char *extension=NULL,*filter=NULL;
        const char *format=demux_format(src->codecpar->codec_id,vob,&extension,&filter);
        if (!format) { ret=AVERROR(ENOSYS); goto done; }
        DemuxOutput *out=&outputs[i];
        if (snprintf(out->path,sizeof(out->path),"%s/track-%02d.%s",directory,i,extension)>=(int)sizeof(out->path)) { ret=AVERROR(ENAMETOOLONG); goto done; }
        if ((ret=avformat_alloc_output_context2(&out->format,NULL,format,out->path))<0) goto done;
        AVStream *dst=avformat_new_stream(out->format,NULL); if (!dst) { ret=AVERROR(ENOMEM); goto done; }
        dst->time_base=src->time_base;
        if (filter) {
            if ((ret=av_bsf_alloc(av_bsf_get_by_name(filter),&out->bsf))<0) goto done;
            if ((ret=avcodec_parameters_copy(out->bsf->par_in,src->codecpar))<0) goto done;
            out->bsf->time_base_in=src->time_base;
            if ((ret=av_bsf_init(out->bsf))<0) goto done;
        }
        if ((ret=avcodec_parameters_copy(dst->codecpar,out->bsf ? out->bsf->par_out : src->codecpar))<0) goto done;
        dst->codecpar->codec_tag=0;
        out->format->interrupt_callback=(AVIOInterruptCB){is_cancelled,cancel};
        out->format->max_delay=0;
        AVDictionary *options=NULL; if (!strcmp(format,"vob")) av_dict_set(&options,"preload","0",0);
        ret=avio_open2(&out->format->pb,out->path,AVIO_FLAG_WRITE,&out->format->interrupt_callback,NULL);
        if (ret>=0) ret=avformat_write_header(out->format,&options);
        av_dict_free(&options); if (ret<0) goto done;
    }
    while ((ret=read_packet(reader,packet))>=0) {
        if (is_cancelled(cancel)) { ret=AVERROR_EXIT; goto done; }
        if (packet->pos >= 0 && packet->pos > processed) processed = packet->pos;
        int percent = input_size > 0 ? (int)(100.0 * processed / input_size) : 0;
        if (percent > 99) percent = 99;
        if (percent > last_percent) {
            report_progress(cancel->env, cancel->engine, progress, percent);
            last_percent = percent;
        }
        int i=packet->stream_index; DemuxOutput *out=&outputs[i];
        if (!out->format) { av_packet_unref(packet); continue; }
        AVRational tb=input->streams[i]->time_base;
        if (out->bsf) {
            if ((ret=av_bsf_send_packet(out->bsf,packet))<0) goto done;
            while ((ret=av_bsf_receive_packet(out->bsf,filtered))>=0) { if ((ret=demux_write(out,filtered,out->bsf->time_base_out))<0) goto done; }
            if (ret!=AVERROR(EAGAIN) && ret!=AVERROR_EOF) goto done;
        } else if ((ret=demux_write(out,packet,tb))<0) goto done;
        av_packet_unref(packet);
    }
    if (ret!=AVERROR_EOF) goto done;
    for (unsigned i=0;i<input->nb_streams;++i) {
        DemuxOutput *out=&outputs[i]; if (!out->format) continue;
        if (out->bsf) {
            if ((ret=av_bsf_send_packet(out->bsf,NULL))<0) goto done;
            while ((ret=av_bsf_receive_packet(out->bsf,filtered))>=0) { if ((ret=demux_write(out,filtered,out->bsf->time_base_out))<0) goto done; }
            if (ret!=AVERROR_EOF && ret!=AVERROR(EAGAIN)) goto done;
        }
        if (!out->count) { ret=AVERROR_INVALIDDATA; goto done; }
        if ((ret=av_write_trailer(out->format))<0) goto done;
        if ((ret=avio_closep(&out->format->pb))<0) goto done;
        if (input->streams[i]->codecpar->codec_id==AV_CODEC_ID_DVD_SUBTITLE) {
            AVDictionaryEntry *lang=av_dict_get(input->streams[i]->metadata,"language",NULL,0);
            if ((ret=demux_vobsub_index(out->path,input->streams[i]->codecpar,lang ? lang->value : NULL))<0) goto done;
        }
    }
    if (chapters && input->nb_chapters) {
        char filename[4096]; snprintf(filename,sizeof(filename),"%s/Chapters.txt",directory);
        FILE *file=fopen(filename,"wb"); if (!file) { ret=AVERROR(EIO); goto done; }
        for (unsigned i=0;i<input->nb_chapters;++i) {
            AVChapter *ch=input->chapters[i]; AVDictionaryEntry *label=av_dict_get(ch->metadata,"title",NULL,0);
            fprintf(file,"CHAPTER%02u=",i+1); demux_time(file,av_rescale_q(ch->start,ch->time_base,(AVRational){1,1000}),'.');
            fprintf(file,"\nCHAPTER%02uNAME=",i+1);
            if (label) { for (const char *p=label->value;*p;++p) fputc(*p=='\r'||*p=='\n' ? ' ' : *p,file); }
            else fprintf(file,"Chapter %u",i+1);
            fputc('\n',file);
        }
        ret=ferror(file) ? AVERROR(EIO) : 0; if (fclose(file)) ret=AVERROR(EIO); if (ret<0) goto done;
    }
    ret=0;
    report_progress(cancel->env, cancel->engine, progress, 100);
 done:
    if (outputs) { for (unsigned i=0;input && i<input->nb_streams;++i) { av_bsf_free(&outputs[i].bsf); if (outputs[i].format) { avio_closep(&outputs[i].format->pb); avformat_free_context(outputs[i].format); } } }
    av_free(outputs); av_packet_free(&packet); av_packet_free(&filtered); return ret;
}

static int demux_media(const char *path, const char *directory, const int *selected, int selected_count, int chapters, int vob, CancelContext *cancel, jmethodID progress)
{
    /* DVD inputs use nativeDemux. This reader receives the MKV itself and
       preserves its timestamps without DVD probing limits or forced GENPTS. */
    AVFormatContext *input = avformat_alloc_context();
    if (!input) return AVERROR(ENOMEM);
    input->interrupt_callback = (AVIOInterruptCB){is_cancelled, cancel};
    int ret = avformat_open_input(&input, path, NULL, NULL);
    if (ret >= 0) ret = avformat_find_stream_info(input, NULL);
    if (ret >= 0) ret = demux_input(input, demux_file_read, input, avio_size(input->pb),
        directory, selected, selected_count, chapters, vob, cancel, progress);
    avformat_close_input(&input);
    return ret;
}

/* Reads the selected DVD cells directly; no intermediate Matroska container. */
typedef struct {
    SourceContext *source;
    AVFormatContext *metadata, *input;
    AVIOContext *io;
    DvdClockSegment *segments;
    int count, current;
    int64_t origin;
} DvdDemuxReader;

static void dvd_demux_close_segment(DvdDemuxReader *reader)
{
    avformat_close_input(&reader->input);
    if (reader->io) { av_freep(&reader->io->buffer); avio_context_free(&reader->io); }
}

static int dvd_demux_read(void *opaque, AVPacket *packet)
{
    DvdDemuxReader *r = opaque;
    while (r->current < r->count) {
        if (is_cancelled(r->source->cancel)) return AVERROR_EXIT;
        if (!r->input) {
            int ret = open_clock_segment(r->source, r->segments[r->current], &r->input, &r->io);
            if (ret < 0) return ret;
            if (!r->current && r->input->start_time != AV_NOPTS_VALUE) r->origin = r->input->start_time;
        }
        int ret = av_read_frame(r->input, packet);
        if (ret == AVERROR_EOF) {
            if (r->io->error < 0 && r->io->error != AVERROR_EOF) return r->io->error;
            dvd_demux_close_segment(r);
            ++r->current;
            continue;
        }
        if (ret < 0) return ret;
        AVStream *src = r->input->streams[packet->stream_index];
        int mapped = -1;
        for (unsigned i = 0; i < r->metadata->nb_streams; ++i) {
            AVStream *dst = r->metadata->streams[i];
            if (src->id == dst->id && src->codecpar->codec_id == dst->codecpar->codec_id) { mapped = i; break; }
        }
        if (mapped < 0) { av_packet_unref(packet); continue; }
        int64_t shift = av_rescale_q(r->segments[r->current].offset, (AVRational){1,90000}, src->time_base)
            - av_rescale_q(r->origin, AV_TIME_BASE_Q, src->time_base);
        if (packet->pts != AV_NOPTS_VALUE) packet->pts += shift;
        if (packet->dts != AV_NOPTS_VALUE) packet->dts += shift;
        av_packet_rescale_ts(packet, src->time_base, r->metadata->streams[mapped]->time_base);
        packet->stream_index = mapped;
        if (packet->pos >= 0) packet->pos += r->segments[r->current].start;
        return 0;
    }
    return AVERROR_EOF;
}

JNIEXPORT jstring JNICALL Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeDemux(
    JNIEnv *env, jobject self, jintArray fds, jlongArray starts, jlongArray ends,
    jstring destination, jlongArray chapter_starts, jlongArray chapter_ends,
    jintArray selection, jlong iso, jint title_set, jobjectArray languages, jintArray palette, jboolean vob)
{
    CancelContext cancel = {env, self, (*env)->GetMethodID(env, (*env)->GetObjectClass(env,self), "isNativeCancelled", "()Z")};
    if (!cancel.method) return NULL;
    jmethodID progress = (*env)->GetMethodID(env, (*env)->GetObjectClass(env,self), "onNativeProgress", "(I)V");
    if (!progress) return NULL;
    char error[512] = {0};
    SourceContext source = {0};
    DvdDemuxReader reader = {.source = &source};
    AVIOContext *metadata_io = NULL;
    int *selected = NULL, selected_count = 0;
    const char *directory = NULL;
    int ret = 0;
    if (is_cancelled(&cancel)) { ret = AVERROR_EXIT; goto done; }
    ret = init_source(env, fds, starts, ends, &source, (DvdUdfSource *)(intptr_t)iso, title_set, &cancel, error, sizeof(error));
    if (ret < 0) goto done;
    report_progress(env, self, progress, 0);
    if (is_cancelled(&cancel)) { ret = AVERROR_EXIT; goto done; }
    directory = (*env)->GetStringUTFChars(env, destination, NULL);
    if (!directory) { ret = AVERROR(ENOMEM); goto done; }
    ret = dvd_clock_segments(&source, &reader.segments, &reader.count);
    if (ret < 0) goto done;
    ret = open_clock_segment(&source, (DvdClockSegment){0, source.stream_size, 0}, &reader.metadata, &metadata_io);
    if (ret < 0) goto done;
    if ((ret = apply_dvd_ifo_metadata(env, reader.metadata, languages, palette)) < 0) goto done;
    if ((ret = add_chapters(env, reader.metadata, chapter_starts, chapter_ends)) < 0) goto done;
    if (reader.metadata->start_time != AV_NOPTS_VALUE) reader.origin = reader.metadata->start_time;
    selected_count = selection ? (*env)->GetArrayLength(env, selection) : (int)reader.metadata->nb_streams;
    selected = av_malloc_array(selected_count, sizeof(*selected));
    if (!selected) { ret = AVERROR(ENOMEM); goto done; }
    if (selection) {
        (*env)->GetIntArrayRegion(env, selection, 0, selected_count, selected);
        if ((*env)->ExceptionCheck(env)) { ret = AVERROR(EINVAL); goto done; }
    } else {
        selected_count = 0;
        for (unsigned i = 0; i < reader.metadata->nb_streams; ++i) {
            enum AVMediaType type = reader.metadata->streams[i]->codecpar->codec_type;
            if (type == AVMEDIA_TYPE_VIDEO || type == AVMEDIA_TYPE_AUDIO || type == AVMEDIA_TYPE_SUBTITLE)
                selected[selected_count++] = i;
        }
    }
    /* The metadata context owns no packet reader after discovery. Close its
       custom IO before opening segments against the same SourceContext. */
    avformat_flush(reader.metadata);
    reader.metadata->pb = NULL;
    av_freep(&metadata_io->buffer); avio_context_free(&metadata_io);
    ret = demux_input(reader.metadata, dvd_demux_read, &reader, source.stream_size,
        directory, selected, selected_count, 1, vob, &cancel, progress);
 done:
    dvd_demux_close_segment(&reader);
    avformat_close_input(&reader.metadata);
    if (metadata_io) { av_freep(&metadata_io->buffer); avio_context_free(&metadata_io); }
    av_free(reader.segments); av_free(selected); free_source(&source);
    if (directory) (*env)->ReleaseStringUTFChars(env, destination, directory);
    if (ret >= 0) return NULL;
    if (ret == AVERROR_EXIT) snprintf(error, sizeof(error), "Demux cancelled");
    if (!error[0]) ff_error(error, sizeof(error), "DVD demux failed", ret);
    return (*env)->NewStringUTF(env, error);
}

JNIEXPORT jstring JNICALL Java_io_github_maas3n_mattmux_AdvancedMergerNative_demux(JNIEnv *env,jobject self,jstring source,jstring destination,jintArray selection,jboolean chapters,jboolean vob)
{
    if (!source || !destination || !selection) return (*env)->NewStringUTF(env,"Choose source, output, and streams");
    const char *path=(*env)->GetStringUTFChars(env,source,NULL), *directory=(*env)->GetStringUTFChars(env,destination,NULL);
    jint *indexes=(*env)->GetIntArrayElements(env,selection,NULL);
    if (!path || !directory || !indexes) { if (path) (*env)->ReleaseStringUTFChars(env,source,path); if (directory) (*env)->ReleaseStringUTFChars(env,destination,directory); if (indexes) (*env)->ReleaseIntArrayElements(env,selection,indexes,JNI_ABORT); return NULL; }
    CancelContext cancel={env,self,(*env)->GetMethodID(env,(*env)->GetObjectClass(env,self),"isNativeCancelled","()Z")};
    jmethodID progress=(*env)->GetMethodID(env,(*env)->GetObjectClass(env,self),"onNativeProgress","(I)V");
    if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); progress=NULL; }
    int ret=demux_media(path,directory,indexes,(*env)->GetArrayLength(env,selection),chapters,vob,&cancel,progress);
    (*env)->ReleaseStringUTFChars(env,source,path); (*env)->ReleaseStringUTFChars(env,destination,directory); (*env)->ReleaseIntArrayElements(env,selection,indexes,JNI_ABORT);
    if (ret<0) { char error[256]; av_strerror(ret,error,sizeof(error)); return (*env)->NewStringUTF(env,error); }
    return NULL;
}
