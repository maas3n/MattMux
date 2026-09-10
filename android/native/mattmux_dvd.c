#include "mattmux_dvd.h"

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MM_MAX_IFO_SIZE (64u << 20)
#define MM_MAX_TITLES 999

typedef struct { uint16_t pgcn; uint16_t pgn; } mm_ptt;
typedef struct { mm_ptt *items; size_t count; } mm_ptt_list;
typedef struct {
    int programs;
    int cells;
    uint8_t playback_mode;
    uint8_t still_time;
    uint8_t *program_map;
    uint8_t *cell_data;
} mm_pgc;

static void set_error(char *error, size_t size, const char *fmt, ...)
{
    if (!error || size == 0) return;
    va_list ap; va_start(ap, fmt); vsnprintf(error, size, fmt, ap); va_end(ap);
}

static uint16_t be16(const uint8_t *b, size_t len, size_t off, int *ok)
{
    if (off + 2 > len) { *ok = 0; return 0; }
    return (uint16_t)((uint16_t)b[off] << 8 | b[off + 1]);
}

static uint32_t be32(const uint8_t *b, size_t len, size_t off, int *ok)
{
    if (off + 4 > len) { *ok = 0; return 0; }
    return (uint32_t)b[off] << 24 |
           (uint32_t)b[off + 1] << 16 |
           (uint32_t)b[off + 2] << 8 |
           (uint32_t)b[off + 3];
}

static int sector_table(const uint8_t *b,size_t len,size_t pointer_off,size_t *base,size_t *end,char *error,size_t error_size)
{
    int ok=1; uint32_t sector=be32(b,len,pointer_off,&ok);
    if(!ok||sector==0){set_error(error,error_size,"IFO table pointer at 0x%zx is invalid",pointer_off);return -1;}
    uint64_t base64=(uint64_t)sector*MM_DVD_SECTOR_SIZE;
    if(base64+8>len){set_error(error,error_size,"IFO table at sector %u is outside file",sector);return -1;}
    uint32_t end_addr=be32(b,len,(size_t)base64+4,&ok); uint64_t end64=base64+(uint64_t)end_addr+1;
    if(!ok||end64>len||end64<base64+8){set_error(error,error_size,"IFO table at sector %u has invalid end address",sector);return -1;}
    *base=(size_t)base64;*end=(size_t)end64;return 0;
}

static int read_ifo(mm_source *source,const char *name,const char *magic,uint8_t **out,size_t *out_size,char *error,size_t error_size)
{
    mm_file *file=NULL;if(mm_source_open_file(source,name,&file,error,error_size)<0)return -1;
    int64_t size64=mm_file_size(file);if(size64<MM_DVD_SECTOR_SIZE||size64>MM_MAX_IFO_SIZE){mm_file_close(file);set_error(error,error_size,"%s has implausible size",name);return -1;}
    size_t size=(size_t)size64;uint8_t *data=malloc(size);if(!data){mm_file_close(file);set_error(error,error_size,"out of memory reading %s",name);return -1;}
    ssize_t got=mm_file_read_at(file,data,size,0);mm_file_close(file);if(got!=(ssize_t)size){free(data);set_error(error,error_size,"could not read complete %s",name);return -1;}
    size_t ml=strlen(magic);if(size<ml||memcmp(data,magic,ml)!=0){free(data);set_error(error,error_size,"%s is not a valid DVD IFO",name);return -1;}
    *out=data;*out_size=size;return 0;
}

static int map_global_title(const uint8_t *vmg,size_t vmg_size,int global_title,int *title_set,int *title_set_title,int *declared_chapters,int *total_titles,char *error,size_t error_size)
{
    size_t base,end;if(sector_table(vmg,vmg_size,0xC4,&base,&end,error,error_size)<0)return -1;int ok=1;uint16_t n=be16(vmg,vmg_size,base,&ok);
    if(!ok||n==0||n>MM_MAX_TITLES){set_error(error,error_size,"DVD title table has invalid title count");return -1;}if(total_titles)*total_titles=n;
    if(global_title<1||global_title>n){set_error(error,error_size,"DVD title %d does not exist (disc has %u titles)",global_title,n);return -1;}
    size_t entry=base+8+(size_t)(global_title-1)*12;if(entry+12>end){set_error(error,error_size,"DVD title %d entry is truncated",global_title);return -1;}
    uint16_t chapters=be16(vmg,vmg_size,entry+2,&ok);int ts=vmg[entry+6],tst=vmg[entry+7];if(!ok||ts<1||ts>99||tst<1){set_error(error,error_size,"DVD title %d has invalid VTS mapping",global_title);return -1;}
    *title_set=ts;*title_set_title=tst;*declared_chapters=chapters;return 0;
}

static void ptt_table_free(mm_ptt_list *table,size_t count){if(!table)return;for(size_t i=0;i<count;i++)free(table[i].items);free(table);}

static int parse_ptt_table(const uint8_t *vts,size_t vts_size,mm_ptt_list **out,size_t *out_count,char *error,size_t error_size)
{
    size_t base,end;if(sector_table(vts,vts_size,0xC8,&base,&end,error,error_size)<0)return -1;int ok=1;uint16_t n16=be16(vts,vts_size,base,&ok);size_t n=n16;
    if(!ok||n==0||n>MM_MAX_TITLES){set_error(error,error_size,"VTS chapter table has invalid title count");return -1;}size_t offsets_end=base+8+n*4;if(offsets_end>end){set_error(error,error_size,"VTS chapter offset table is truncated");return -1;}
    uint32_t *offsets=calloc(n,sizeof(*offsets));mm_ptt_list *table=calloc(n,sizeof(*table));if(!offsets||!table){free(offsets);free(table);set_error(error,error_size,"out of memory parsing DVD chapters");return -1;}
    for(size_t i=0;i<n;i++){offsets[i]=be32(vts,vts_size,base+8+i*4,&ok);if(!ok){free(offsets);ptt_table_free(table,n);set_error(error,error_size,"VTS chapter offset is truncated");return -1;}}
    for(size_t i=0;i<n;i++){
        size_t start=base+offsets[i],stop=i+1<n?base+offsets[i+1]:end;if(start<offsets_end||stop<start||stop>end||(stop-start)%4!=0){free(offsets);ptt_table_free(table,n);set_error(error,error_size,"VTS title %zu has invalid chapter offsets",i+1);return -1;}
        size_t count=(stop-start)/4;table[i].items=calloc(count,sizeof(*table[i].items));if(count&&!table[i].items){free(offsets);ptt_table_free(table,n);set_error(error,error_size,"out of memory parsing DVD chapters");return -1;}table[i].count=count;
        for(size_t j=0;j<count;j++){table[i].items[j].pgcn=be16(vts,vts_size,start+j*4,&ok);table[i].items[j].pgn=be16(vts,vts_size,start+j*4+2,&ok);if(!ok){free(offsets);ptt_table_free(table,n);set_error(error,error_size,"VTS chapter entry is truncated");return -1;}}
    }
    free(offsets);*out=table;*out_count=n;return 0;
}

static void pgc_free(mm_pgc *pgc){if(!pgc)return;free(pgc->program_map);free(pgc->cell_data);memset(pgc,0,sizeof(*pgc));}

static int parse_pgc(const uint8_t *vts,size_t vts_size,uint16_t pgcn,mm_pgc *out,char *error,size_t error_size)
{
    size_t base,end;if(sector_table(vts,vts_size,0xCC,&base,&end,error,error_size)<0)return -1;int ok=1;uint16_t n=be16(vts,vts_size,base,&ok);if(!ok||pgcn<1||pgcn>n){set_error(error,error_size,"PGC %u is outside VTS program chain table",pgcn);return -1;}
    size_t srp=base+8+(size_t)(pgcn-1)*8;if(srp+8>end){set_error(error,error_size,"PGC %u descriptor is truncated",pgcn);return -1;}uint32_t rel=be32(vts,vts_size,srp+4,&ok);size_t pgc_base=base+rel;if(!ok||pgc_base<base||pgc_base+0xEC>end){set_error(error,error_size,"PGC %u header is outside VTS table",pgcn);return -1;}
    out->programs=vts[pgc_base+0x02];out->cells=vts[pgc_base+0x03];out->still_time=vts[pgc_base+0xA2];out->playback_mode=vts[pgc_base+0xA3];
    if(out->programs<1||out->cells<1||out->programs>out->cells){set_error(error,error_size,"PGC %u has invalid program/cell counts",pgcn);return -1;}
    if(out->playback_mode!=0||out->still_time!=0){set_error(error,error_size,"DVD title uses shuffle/random or still-time authoring not supported by the LGPL engine");return -1;}
    uint16_t program_rel=be16(vts,vts_size,pgc_base+0xE6,&ok),cell_rel=be16(vts,vts_size,pgc_base+0xE8,&ok);if(!ok||!program_rel||!cell_rel){set_error(error,error_size,"PGC %u has no program/cell table",pgcn);return -1;}
    size_t ps=pgc_base+program_rel,cs=pgc_base+cell_rel,pe=ps+(size_t)out->programs,ce=cs+(size_t)out->cells*24;if(ps<pgc_base||pe>end||cs<pgc_base||ce>end){set_error(error,error_size,"PGC %u program/cell table is truncated",pgcn);return -1;}
    out->program_map=malloc((size_t)out->programs);out->cell_data=malloc((size_t)out->cells*24);if(!out->program_map||!out->cell_data){pgc_free(out);set_error(error,error_size,"out of memory parsing PGC");return -1;}
    memcpy(out->program_map,vts+ps,(size_t)out->programs);memcpy(out->cell_data,vts+cs,(size_t)out->cells*24);int prev=0;for(int i=0;i<out->programs;i++){int cell=out->program_map[i];if(cell<1||cell>out->cells||cell<=prev){pgc_free(out);set_error(error,error_size,"PGC %u program map is not sequential",pgcn);return -1;}prev=cell;}return 0;
}

static int decode_bcd(uint8_t b,int *value){int hi=b>>4,lo=b&0x0f;if(hi>9||lo>9)return -1;*value=hi*10+lo;return 0;}
static int decode_dvd_time_ms(const uint8_t *b,int64_t *out)
{
    int hh,mm,ss;if(decode_bcd(b[0],&hh)<0||decode_bcd(b[1],&mm)<0||decode_bcd(b[2],&ss)<0||mm>=60||ss>=60)return -1;int rate=b[3]>>6,frames=((b[3]>>4)&3)*10+(b[3]&15);int64_t ms=((int64_t)hh*3600+(int64_t)mm*60+ss)*1000;
    if(rate==1){if(frames>=25)return -1;ms+=frames*1000/25;}else if(rate==3){if(frames>=30)return -1;ms+=frames*1000/30;}else if(frames!=0)return -1;*out=ms;return 0;
}

static int cell_is_selected_angle(const uint8_t *entry,char *error,size_t error_size)
{
    (void)error;(void)error_size;int block_mode=(entry[0]>>6)&3;return block_mode==0||block_mode==1;
}

static int program_timeline(const mm_pgc *pgc,int64_t **out_starts,int64_t *out_total,char *error,size_t error_size)
{
    int64_t *starts=calloc((size_t)pgc->programs,sizeof(*starts));if(!starts){set_error(error,error_size,"out of memory building DVD timeline");return -1;}int64_t total=0;
    for(int p=0;p<pgc->programs;p++){starts[p]=total;int first=pgc->program_map[p],last=p+1<pgc->programs?pgc->program_map[p+1]-1:pgc->cells;if(first<1||last<first||last>pgc->cells){free(starts);set_error(error,error_size,"invalid cell span for DVD program %d",p+1);return -1;}
        for(int cell=first;cell<=last;cell++){const uint8_t *entry=pgc->cell_data+(size_t)(cell-1)*24;if(!cell_is_selected_angle(entry,error,error_size))continue;if(entry[2]!=0){free(starts);set_error(error,error_size,"DVD cell %d uses still-time semantics",cell);return -1;}int64_t d;if(decode_dvd_time_ms(entry+4,&d)<0){free(starts);set_error(error,error_size,"DVD cell %d has invalid playback time",cell);return -1;}total+=d;}}
    if(total<=0){free(starts);set_error(error,error_size,"DVD program duration is zero");return -1;}*out_starts=starts;*out_total=total;return 0;
}

static int append_cell(mm_title_plan *plan,const uint8_t *entry,int64_t timeline,char *error,size_t error_size)
{
    int ok=1;uint32_t first=be32(entry,24,8,&ok),last=be32(entry,24,20,&ok);int64_t duration;if(!ok||last<first||decode_dvd_time_ms(entry+4,&duration)<0||duration<0){set_error(error,error_size,"DVD cell has invalid sector range or duration");return -1;}
    mm_cell *next=realloc(plan->cells,(plan->cell_count+1)*sizeof(*next));if(!next){set_error(error,error_size,"out of memory building DVD cell list");return -1;}plan->cells=next;plan->cells[plan->cell_count++]=(mm_cell){first,last,timeline,duration};return 0;
}

void mm_title_plan_free(mm_title_plan *plan){if(!plan)return;free(plan->chapters);free(plan->cells);memset(plan,0,sizeof(*plan));}

int mm_build_title_plan(mm_source *source,int global_title,mm_title_plan *out,char *error,size_t error_size)
{
    if(!source||!out){set_error(error,error_size,"invalid title plan request");return -1;}memset(out,0,sizeof(*out));uint8_t *vmg=NULL,*vts=NULL;size_t vmg_size=0,vts_size=0;mm_ptt_list *ptt=NULL;size_t ptt_count=0;mm_pgc pgc={0};int64_t *starts=NULL;int rc=-1;
    if(read_ifo(source,"VIDEO_TS.IFO","DVDVIDEO-VMG",&vmg,&vmg_size,error,error_size)<0)goto done;int ts,tst,declared;if(map_global_title(vmg,vmg_size,global_title,&ts,&tst,&declared,NULL,error,error_size)<0)goto done;
    char vts_name[32];snprintf(vts_name,sizeof(vts_name),"VTS_%02d_0.IFO",ts);if(read_ifo(source,vts_name,"DVDVIDEO-VTS",&vts,&vts_size,error,error_size)<0)goto done;if(parse_ptt_table(vts,vts_size,&ptt,&ptt_count,error,error_size)<0)goto done;if(tst<1||(size_t)tst>ptt_count||ptt[tst-1].count==0){set_error(error,error_size,"DVD title has no usable chapter/program mapping");goto done;}
    mm_ptt_list *chap=&ptt[tst-1];uint16_t pgcn=chap->items[0].pgcn,prev=0;if(!pgcn){set_error(error,error_size,"DVD title has invalid PGC 0");goto done;}for(size_t i=0;i<chap->count;i++){if(chap->items[i].pgcn!=pgcn||!chap->items[i].pgn||(i&&chap->items[i].pgn<=prev)){set_error(error,error_size,"DVD title uses branching/non-sequential chapter authoring not supported by the LGPL engine");goto done;}prev=chap->items[i].pgn;}
    if(parse_pgc(vts,vts_size,pgcn,&pgc,error,error_size)<0)goto done;int64_t total;if(program_timeline(&pgc,&starts,&total,error,error_size)<0)goto done;
    int first=chap->items[0].pgn,last_chapter=chap->items[chap->count-1].pgn;if(first<1||first>pgc.programs||last_chapter>pgc.programs){set_error(error,error_size,"DVD chapter references a program outside its PGC");goto done;}int end=pgc.programs+1;for(size_t i=0;i<ptt_count;i++){if((int)i==tst-1||ptt[i].count==0)continue;mm_ptt other=ptt[i].items[0];if(other.pgcn==pgcn&&other.pgn>last_chapter&&other.pgn<end)end=other.pgn;}if(end<=first||end>pgc.programs+1){set_error(error,error_size,"DVD title has invalid program bounds");goto done;}
    int64_t base=starts[first-1],title_end=end<=pgc.programs?starts[end-1]:total;if(title_end<=base){set_error(error,error_size,"DVD title has invalid duration");goto done;}if(declared>0&&declared!=(int)chap->count){set_error(error,error_size,"DVD chapter count disagrees between VMG and VTS tables");goto done;}
    out->number=global_title;out->title_set=ts;out->title_set_title=tst;out->pgcn=pgcn;out->duration_ms=title_end-base;out->chapter_count=chap->count;out->chapters=calloc(chap->count,sizeof(*out->chapters));if(!out->chapters){set_error(error,error_size,"out of memory building chapter list");goto done;}
    for(size_t i=0;i<chap->count;i++){int pgn=chap->items[i].pgn;if(pgn<first||pgn>=end){set_error(error,error_size,"DVD chapter starts outside selected title");goto done;}int64_t st=starts[pgn-1]-base,en=title_end-base;if(i+1<chap->count)en=starts[chap->items[i+1].pgn-1]-base;if(en<=st){set_error(error,error_size,"DVD chapter has non-positive duration");goto done;}out->chapters[i]=(mm_chapter){(int)i+1,st,en-st};}
    int64_t timeline=0;for(int pgn=first;pgn<end;pgn++){int fc=pgc.program_map[pgn-1],lc=pgn<pgc.programs?pgc.program_map[pgn]-1:pgc.cells;for(int cell=fc;cell<=lc;cell++){const uint8_t *entry=pgc.cell_data+(size_t)(cell-1)*24;if(!cell_is_selected_angle(entry,error,error_size))continue;if(entry[2]!=0){set_error(error,error_size,"DVD cell %d uses still-time semantics",cell);goto done;}if(append_cell(out,entry,timeline,error,error_size)<0)goto done;timeline+=out->cells[out->cell_count-1].duration_ms;}}
    if(!out->cell_count){set_error(error,error_size,"DVD title has no readable cells");goto done;}if(llabs(timeline-out->duration_ms)>1000){set_error(error,error_size,"DVD title cell timeline does not match its program timeline");goto done;}out->duration_ms=timeline;rc=0;
done:free(vmg);free(vts);ptt_table_free(ptt,ptt_count);pgc_free(&pgc);free(starts);if(rc<0)mm_title_plan_free(out);return rc;
}

int mm_scan_titles(mm_source *source,mm_title_summary **out_titles,size_t *out_count,char *error,size_t error_size)
{
    if(!source||!out_titles||!out_count){set_error(error,error_size,"invalid title scan request");return -1;}*out_titles=NULL;*out_count=0;uint8_t *vmg=NULL;size_t vmg_size=0;if(read_ifo(source,"VIDEO_TS.IFO","DVDVIDEO-VMG",&vmg,&vmg_size,error,error_size)<0)return -1;int total=0,a,b,c;if(map_global_title(vmg,vmg_size,1,&a,&b,&c,&total,error,error_size)<0){free(vmg);return -1;}free(vmg);
    mm_title_summary *titles=NULL;size_t count=0;char last[MM_MAX_ERROR]={0};for(int title=1;title<=total;title++){mm_title_plan plan;char local[MM_MAX_ERROR]={0};if(mm_build_title_plan(source,title,&plan,local,sizeof(local))<0){snprintf(last,sizeof(last),"Title %d: %.980s",title,local[0]?local:"unsupported DVD authoring");continue;}mm_title_summary *next=realloc(titles,(count+1)*sizeof(*next));if(!next){mm_title_plan_free(&plan);free(titles);set_error(error,error_size,"out of memory scanning DVD titles");return -1;}titles=next;titles[count++]=(mm_title_summary){title,plan.duration_ms,(int)plan.chapter_count};mm_title_plan_free(&plan);}
    if(!count){free(titles);set_error(error,error_size,"%s",last[0]?last:"No readable DVD titles found");return -1;}*out_titles=titles;*out_count=count;return 0;
}
