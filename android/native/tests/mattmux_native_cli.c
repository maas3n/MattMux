#include "../mattmux_dvd.h"
#include "../mattmux_remux.h"

#include <dirent.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#include <unistd.h>

static int suffix(const char *s, const char *x) { size_t a=strlen(s),b=strlen(x); return a>=b && strcasecmp(s+a-b,x)==0; }
static int dvd_name(const char *name)
{
    if (!strcasecmp(name,"VIDEO_TS.IFO")) return 1;
    if (strncasecmp(name,"VTS_",4)) return 0;
    size_t n=strlen(name);
    if (n==12 && suffix(name,"_0.IFO")) return 1;
    return n==12 && suffix(name,".VOB") && name[6]=='_' && name[7]>='1' && name[7]<='9';
}
static mm_source *open_folder(const char *path,char *error,size_t error_size)
{
    char dirpath[4096]; snprintf(dirpath,sizeof(dirpath),"%s/VIDEO_TS",path);
    DIR *dir=opendir(dirpath); if(!dir){snprintf(dirpath,sizeof(dirpath),"%s",path);dir=opendir(dirpath);} if(!dir){snprintf(error,error_size,"could not open VIDEO_TS directory");return NULL;}
    char *names[256]={0}; int fds[256]={0}; size_t count=0; struct dirent *e;
    while((e=readdir(dir)) && count<256){if(!dvd_name(e->d_name))continue;char full[8192];snprintf(full,sizeof(full),"%s/%s",dirpath,e->d_name);int fd=open(full,O_RDONLY);if(fd<0)continue;names[count]=strdup(e->d_name);fds[count]=fd;if(!names[count]){close(fd);break;}count++;}
    closedir(dir); if(!count){snprintf(error,error_size,"no DVD files found");return NULL;}
    const char *cn[256]; for(size_t i=0;i<count;i++)cn[i]=names[i]; mm_source *src=NULL; int rc=mm_source_init_files(&src,cn,fds,count,error,error_size); for(size_t i=0;i<count;i++){free(names[i]);close(fds[i]);} return rc==0?src:NULL;
}
static int progress(void *opaque,int percent){(void)opaque;fprintf(stderr,"progress=%d\n",percent);return 0;}
int main(int argc,char **argv)
{
    if(argc!=3){fprintf(stderr,"usage: %s <VIDEO_TS-root-or-iso> <output.mkv>\n",argv[0]);return 2;}
    char error[MM_MAX_ERROR]={0}; mm_source *src=NULL;
    if(suffix(argv[1],".iso")){int fd=open(argv[1],O_RDONLY);if(fd<0||mm_source_init_iso(&src,fd,error,sizeof(error))<0){if(fd>=0)close(fd);fprintf(stderr,"source error: %s\n",error);return 1;}close(fd);}else{src=open_folder(argv[1],error,sizeof(error));if(!src){fprintf(stderr,"source error: %s\n",error);return 1;}}
    mm_title_summary *titles=NULL;size_t n=0;if(mm_scan_titles(src,&titles,&n,error,sizeof(error))<0){fprintf(stderr,"scan error: %s\n",error);mm_source_close(src);return 1;}size_t sel=0;for(size_t i=1;i<n;i++)if(titles[i].duration_ms>titles[sel].duration_ms)sel=i;
    printf("title=%d duration_ms=%lld chapters=%d\n",titles[sel].number,(long long)titles[sel].duration_ms,titles[sel].chapter_count);
    mm_title_plan plan;if(mm_build_title_plan(src,titles[sel].number,&plan,error,sizeof(error))<0){fprintf(stderr,"plan error: %s\n",error);free(titles);mm_source_close(src);return 1;}
    int out=open(argv[2],O_RDWR|O_CREAT|O_TRUNC,0644);if(out<0){perror("open output");mm_title_plan_free(&plan);free(titles);mm_source_close(src);return 1;}
    _Atomic int cancelled=0;int streams=0;int rc=mm_remux_title(src,&plan,out,1,&cancelled,progress,NULL,&streams,error,sizeof(error));close(out);if(rc<0){fprintf(stderr,"remux error: %s\n",error);unlink(argv[2]);}else printf("streams=%d\n",streams);
    mm_title_plan_free(&plan);free(titles);mm_source_close(src);return rc?1:0;
}
