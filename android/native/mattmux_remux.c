#define _FILE_OFFSET_BITS 64
#include "mattmux_remux.h"
#include <errno.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <libavcodec/codec_par.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>
#include <libavutil/dict.h>
#include <libavutil/error.h>
#include <libavutil/mathematics.h>
#include <libavutil/mem.h>
#define MM_IO_BUFFER (64*1024)
#define MM_MAX_VOBS 9

typedef struct{mm_file *file;int64_t start,size;} mm_vob_piece;
typedef struct{mm_vob_piece pieces[MM_MAX_VOBS];size_t count;int64_t total_size;} mm_vob_set;
typedef struct{int64_t physical_start,size,virtual_start;} mm_reader_cell;
typedef struct{mm_vob_set vobs;mm_reader_cell *cells;size_t cell_count;int64_t size,pos,max_pos;_Atomic int *cancelled;} mm_cell_reader;
typedef struct{int fd;_Atomic int *cancelled;} mm_fd_writer;

static void set_error(char *error,size_t size,const char *fmt,...){if(!error||!size)return;va_list ap;va_start(ap,fmt);vsnprintf(error,size,fmt,ap);va_end(ap);}
static void set_av_error(char *error,size_t size,const char *what,int code){char detail[AV_ERROR_MAX_STRING_SIZE]={0};av_strerror(code,detail,sizeof(detail));set_error(error,size,"%s: %s",what,detail[0]?detail:"FFmpeg error");}
static void close_vobs(mm_vob_set *v){if(!v)return;for(size_t i=0;i<v->count;i++)mm_file_close(v->pieces[i].file);memset(v,0,sizeof(*v));}

static int open_vobs(mm_source *source,int title_set,mm_vob_set *out,char *error,size_t error_size)
{
    memset(out,0,sizeof(*out));for(int part=1;part<=MM_MAX_VOBS;part++){char name[32];snprintf(name,sizeof(name),"VTS_%02d_%d.VOB",title_set,part);mm_file *file=NULL;char local[MM_MAX_ERROR]={0};if(mm_source_open_file(source,name,&file,local,sizeof(local))<0){if(part==1){set_error(error,error_size,"%s",local[0]?local:"DVD title VOB was not found");return -1;}break;}int64_t size=mm_file_size(file);if(size<=0){mm_file_close(file);close_vobs(out);set_error(error,error_size,"%s is empty",name);return -1;}out->pieces[out->count++]=(mm_vob_piece){file,out->total_size,size};out->total_size+=size;}return out->count?0:-1;
}

static ssize_t read_physical(mm_vob_set *v,int64_t offset,uint8_t *buf,size_t size)
{
    if(!v||offset<0||!buf)return -1;if(offset>=v->total_size)return 0;if((uint64_t)size>(uint64_t)(v->total_size-offset))size=(size_t)(v->total_size-offset);size_t total=0;
    while(total<size){size_t index=v->count;for(size_t i=0;i<v->count;i++)if(offset>=v->pieces[i].start&&offset<v->pieces[i].start+v->pieces[i].size){index=i;break;}if(index==v->count)return total?(ssize_t)total:-1;mm_vob_piece *p=&v->pieces[index];int64_t within=offset-p->start;size_t chunk=size-total;if((int64_t)chunk>p->size-within)chunk=(size_t)(p->size-within);ssize_t got=mm_file_read_at(p->file,buf+total,chunk,within);if(got<=0)return total?(ssize_t)total:got;total+=(size_t)got;offset+=got;}return (ssize_t)total;
}

static int init_cell_reader(mm_source *source,const mm_title_plan *plan,_Atomic int *cancelled,mm_cell_reader *out,char *error,size_t error_size)
{
    memset(out,0,sizeof(*out));out->cancelled=cancelled;if(open_vobs(source,plan->title_set,&out->vobs,error,error_size)<0)return -1;out->cells=calloc(plan->cell_count,sizeof(*out->cells));if(!out->cells){close_vobs(&out->vobs);set_error(error,error_size,"out of memory creating DVD cell stream");return -1;}out->cell_count=plan->cell_count;int64_t vp=0;
    for(size_t i=0;i<plan->cell_count;i++){uint64_t physical=(uint64_t)plan->cells[i].first_sector*MM_DVD_SECTOR_SIZE,sectors=(uint64_t)plan->cells[i].last_sector-plan->cells[i].first_sector+1,bytes=sectors*MM_DVD_SECTOR_SIZE;if(physical>INT64_MAX||bytes>INT64_MAX||physical+bytes>(uint64_t)out->vobs.total_size){free(out->cells);out->cells=NULL;close_vobs(&out->vobs);set_error(error,error_size,"DVD cell %zu points outside VTS_%02d title VOBs",i+1,plan->title_set);return -1;}out->cells[i]=(mm_reader_cell){(int64_t)physical,(int64_t)bytes,vp};vp+=(int64_t)bytes;}out->size=vp;return 0;
}
static void free_cell_reader(mm_cell_reader *r){if(!r)return;free(r->cells);close_vobs(&r->vobs);memset(r,0,sizeof(*r));}

static int input_read_packet(void *opaque,uint8_t *buf,int buf_size)
{
    mm_cell_reader *r=opaque;if(!r||!buf||buf_size<=0)return AVERROR(EINVAL);if(r->cancelled&&atomic_load(r->cancelled))return AVERROR_EXIT;if(r->pos>=r->size)return AVERROR_EOF;int wanted=buf_size;if((int64_t)wanted>r->size-r->pos)wanted=(int)(r->size-r->pos);int total=0;
    while(total<wanted){size_t index=r->cell_count;for(size_t i=0;i<r->cell_count;i++)if(r->pos>=r->cells[i].virtual_start&&r->pos<r->cells[i].virtual_start+r->cells[i].size){index=i;break;}if(index==r->cell_count)break;mm_reader_cell *c=&r->cells[index];int64_t within=r->pos-c->virtual_start;int chunk=wanted-total;if((int64_t)chunk>c->size-within)chunk=(int)(c->size-within);ssize_t got=read_physical(&r->vobs,c->physical_start+within,buf+total,(size_t)chunk);if(got<0)return total?total:AVERROR(EIO);if(!got)break;r->pos+=got;total+=(int)got;if(r->pos>r->max_pos)r->max_pos=r->pos;if(r->cancelled&&atomic_load(r->cancelled))return total?total:AVERROR_EXIT;}return total?total:AVERROR_EOF;
}
static int64_t input_seek(void *opaque,int64_t offset,int whence){mm_cell_reader *r=opaque;if(!r)return AVERROR(EINVAL);if(whence==AVSEEK_SIZE)return r->size;whence&=~AVSEEK_FORCE;int64_t target;switch(whence){case SEEK_SET:target=offset;break;case SEEK_CUR:target=r->pos+offset;break;case SEEK_END:target=r->size+offset;break;default:return AVERROR(EINVAL);}if(target<0||target>r->size)return AVERROR(EINVAL);r->pos=target;return target;}
static int output_write_packet(void *opaque,const uint8_t *buf,int buf_size){mm_fd_writer *w=opaque;if(!w||w->fd<0||!buf||buf_size<0)return AVERROR(EINVAL);if(w->cancelled&&atomic_load(w->cancelled))return AVERROR_EXIT;int done=0;while(done<buf_size){ssize_t n=write(w->fd,buf+done,(size_t)(buf_size-done));if(n<0){if(errno==EINTR)continue;return AVERROR(errno);}if(!n)return AVERROR(EIO);done+=(int)n;}return done;}
static int64_t output_seek(void *opaque,int64_t offset,int whence){mm_fd_writer *w=opaque;if(!w||w->fd<0)return AVERROR(EINVAL);if(whence==AVSEEK_SIZE){off_t cur=lseek(w->fd,0,SEEK_CUR),end=lseek(w->fd,0,SEEK_END);if(cur>=0)(void)lseek(w->fd,cur,SEEK_SET);return end>=0?(int64_t)end:AVERROR(errno);}whence&=~AVSEEK_FORCE;off_t result=lseek(w->fd,(off_t)offset,whence);return result<0?AVERROR(errno):(int64_t)result;}

static int add_chapters(AVFormatContext *out,const mm_title_plan *plan)
{
    if(!plan->chapter_count)return 0;out->chapters=av_calloc(plan->chapter_count,sizeof(*out->chapters));if(!out->chapters)return AVERROR(ENOMEM);out->nb_chapters=(unsigned)plan->chapter_count;
    for(size_t i=0;i<plan->chapter_count;i++){AVChapter *c=av_mallocz(sizeof(*c));if(!c)return AVERROR(ENOMEM);c->id=(int)i;c->time_base=(AVRational){1,1000};c->start=plan->chapters[i].start_ms;c->end=c->start+plan->chapters[i].duration_ms;char title[32];snprintf(title,sizeof(title),"Chapter %02zu",i+1);av_dict_set(&c->metadata,"title",title,0);out->chapters[i]=c;}return 0;
}
static void maybe_progress(mm_cell_reader *r,mm_progress_fn fn,void *opaque,int *last){if(!fn||!r||r->size<=0)return;int p=(int)((r->max_pos*99)/r->size);if(p>99)p=99;if(p>=*last+1){*last=p;(void)fn(opaque,p);}}

int mm_remux_title(mm_source *source,const mm_title_plan *plan,int output_fd,int preserve_chapters,_Atomic int *cancelled,mm_progress_fn progress,void *progress_opaque,int *out_stream_count,char *error,size_t error_size)
{
    if(!source||!plan||output_fd<0){set_error(error,error_size,"invalid remux request");return -1;}if(cancelled)atomic_store(cancelled,0);if(out_stream_count)*out_stream_count=0;mm_cell_reader reader;if(init_cell_reader(source,plan,cancelled,&reader,error,error_size)<0)return -1;
    AVFormatContext *in=NULL,*out=NULL;AVIOContext *in_io=NULL,*out_io=NULL;AVPacket *packet=NULL;int *map=NULL,writer_fd=-1,header=0,rc=-1,ff=0;uint8_t *in_buffer=av_malloc(MM_IO_BUFFER);if(!in_buffer){set_error(error,error_size,"out of memory allocating FFmpeg input buffer");goto done;}in_io=avio_alloc_context(in_buffer,MM_IO_BUFFER,0,&reader,input_read_packet,NULL,input_seek);if(!in_io){av_free(in_buffer);set_error(error,error_size,"could not create FFmpeg input I/O");goto done;}in_io->seekable=AVIO_SEEKABLE_NORMAL;
    in=avformat_alloc_context();if(!in){set_error(error,error_size,"could not allocate FFmpeg input context");goto done;}in->pb=in_io;in->flags|=AVFMT_FLAG_CUSTOM_IO|AVFMT_FLAG_GENPTS;const AVInputFormat *mpeg=av_find_input_format("mpeg");if(!mpeg){set_error(error,error_size,"bundled FFmpeg does not contain the MPEG-PS demuxer");goto done;}ff=avformat_open_input(&in,"mattmux-dvd-title.vob",mpeg,NULL);if(ff<0){set_av_error(error,error_size,"Could not read selected DVD title (encrypted/damaged discs are not supported)",ff);goto done;}ff=avformat_find_stream_info(in,NULL);if(ff<0){set_av_error(error,error_size,"Could not identify DVD title streams",ff);goto done;}
    ff=avformat_alloc_output_context2(&out,NULL,"matroska",NULL);if(ff<0||!out){set_av_error(error,error_size,"Could not create Matroska output",ff<0?ff:AVERROR_UNKNOWN);goto done;}map=av_malloc_array(in->nb_streams,sizeof(*map));if(!map){set_error(error,error_size,"out of memory creating stream map");goto done;}for(unsigned i=0;i<in->nb_streams;i++)map[i]=-1;int mapped=0;
    for(unsigned i=0;i<in->nb_streams;i++){AVStream *is=in->streams[i];enum AVMediaType type=is->codecpar->codec_type;if(type!=AVMEDIA_TYPE_VIDEO&&type!=AVMEDIA_TYPE_AUDIO&&type!=AVMEDIA_TYPE_SUBTITLE)continue;AVStream *os=avformat_new_stream(out,NULL);if(!os){set_error(error,error_size,"could not allocate Matroska stream");goto done;}ff=avcodec_parameters_copy(os->codecpar,is->codecpar);if(ff<0){set_av_error(error,error_size,"Could not copy DVD stream parameters",ff);goto done;}os->codecpar->codec_tag=0;os->time_base=is->time_base;os->disposition=is->disposition;av_dict_copy(&os->metadata,is->metadata,0);map[i]=mapped++;}
    if(!mapped){set_error(error,error_size,"No video/audio/subtitle streams were found. The DVD may be encrypted or unsupported.");goto done;}if(out_stream_count)*out_stream_count=mapped;av_dict_set(&out->metadata,"encoder","MattMux Android native stream-copy engine",0);if(preserve_chapters&&(ff=add_chapters(out,plan))<0){set_av_error(error,error_size,"Could not create chapter table",ff);goto done;}
    writer_fd=dup(output_fd);if(writer_fd<0){set_error(error,error_size,"could not duplicate output descriptor: %s",strerror(errno));goto done;}if(lseek(writer_fd,0,SEEK_SET)<0||ftruncate(writer_fd,0)<0){set_error(error,error_size,"Output provider is not seekable. Choose local or ChromeOS Files storage.");goto done;}mm_fd_writer writer={writer_fd,cancelled};uint8_t *out_buffer=av_malloc(MM_IO_BUFFER);if(!out_buffer){set_error(error,error_size,"out of memory allocating FFmpeg output buffer");goto done;}out_io=avio_alloc_context(out_buffer,MM_IO_BUFFER,1,&writer,NULL,output_write_packet,output_seek);if(!out_io){av_free(out_buffer);set_error(error,error_size,"could not create FFmpeg output I/O");goto done;}out_io->seekable=AVIO_SEEKABLE_NORMAL;out->pb=out_io;out->flags|=AVFMT_FLAG_CUSTOM_IO;out->avoid_negative_ts=AVFMT_AVOID_NEG_TS_MAKE_ZERO;
    ff=avformat_write_header(out,NULL);if(ff<0){set_av_error(error,error_size,"Could not write Matroska header",ff);goto done;}header=1;packet=av_packet_alloc();if(!packet){set_error(error,error_size,"out of memory allocating media packet");goto done;}int last=-1;if(progress)progress(progress_opaque,0);
    for(;;){if(cancelled&&atomic_load(cancelled)){set_error(error,error_size,"Remux cancelled");goto done;}ff=av_read_frame(in,packet);if(ff==AVERROR_EOF)break;if(ff<0){if(cancelled&&atomic_load(cancelled))set_error(error,error_size,"Remux cancelled");else set_av_error(error,error_size,"Error reading DVD title",ff);goto done;}int ii=packet->stream_index;if(ii<0||(unsigned)ii>=in->nb_streams||map[ii]<0){av_packet_unref(packet);maybe_progress(&reader,progress,progress_opaque,&last);continue;}AVStream *is=in->streams[ii],*os=out->streams[map[ii]];if(in->start_time!=AV_NOPTS_VALUE){int64_t shift=av_rescale_q(in->start_time,AV_TIME_BASE_Q,is->time_base);if(packet->pts!=AV_NOPTS_VALUE)packet->pts-=shift;if(packet->dts!=AV_NOPTS_VALUE)packet->dts-=shift;}av_packet_rescale_ts(packet,is->time_base,os->time_base);packet->stream_index=map[ii];packet->pos=-1;ff=av_interleaved_write_frame(out,packet);av_packet_unref(packet);if(ff<0){set_av_error(error,error_size,"Matroska stream-copy failed (the selected DVD stream may be unsupported)",ff);goto done;}maybe_progress(&reader,progress,progress_opaque,&last);}
    ff=av_write_trailer(out);header=0;if(ff<0){set_av_error(error,error_size,"Could not finalize Matroska output",ff);goto done;}avio_flush(out_io);if(fsync(writer_fd)<0&&errno!=EINVAL&&errno!=ENOTSUP){set_error(error,error_size,"Could not flush output file: %s",strerror(errno));goto done;}if(progress)progress(progress_opaque,100);rc=0;
done:if(packet)av_packet_free(&packet);if(header&&out)(void)av_write_trailer(out);if(out)avformat_free_context(out);if(out_io){av_freep(&out_io->buffer);avio_context_free(&out_io);}if(writer_fd>=0)close(writer_fd);av_free(map);if(in)avformat_close_input(&in);if(in_io){av_freep(&in_io->buffer);avio_context_free(&in_io);}free_cell_reader(&reader);return rc;
}
