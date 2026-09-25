#ifndef MATTMUX_HOST_ANDROID_LOG_H
#define MATTMUX_HOST_ANDROID_LOG_H
#include <stdarg.h>
#include <stdio.h>
#define ANDROID_LOG_INFO 4
#define ANDROID_LOG_ERROR 6
static inline int __android_log_print(int priority, const char *tag, const char *fmt, ...) {
    (void)priority;
    fprintf(stderr, "%s: ", tag);
    va_list args;
    va_start(args, fmt);
    int result = vfprintf(stderr, fmt, args);
    va_end(args);
    fputc('\n', stderr);
    return result;
}
#endif
