#include <jni.h>
#include <stdio.h>

#include <libavcodec/avcodec.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>

static void format_version(char *out, size_t out_size, unsigned version)
{
    snprintf(
        out,
        out_size,
        "%u.%u.%u",
        (version >> 16) & 0xff,
        (version >> 8) & 0xff,
        version & 0xff
    );
}

JNIEXPORT jstring JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeVersionSummary(
    JNIEnv *env,
    jobject thiz
)
{
    (void)thiz;

    char avformat[32];
    char avcodec[32];
    char avutil[32];
    char summary[192];

    format_version(avformat, sizeof(avformat), avformat_version());
    format_version(avcodec, sizeof(avcodec), avcodec_version());
    format_version(avutil, sizeof(avutil), avutil_version());

    snprintf(
        summary,
        sizeof(summary),
        "FFmpeg %s (libavformat %s, libavcodec %s, libavutil %s)",
        av_version_info(),
        avformat,
        avcodec,
        avutil
    );

    return (*env)->NewStringUTF(env, summary);
}
