/* Host-only log adapter for tests that compile the production Android DVD JNI. */
#ifndef MATTMUX_TEST_ANDROID_LOG_H
#define MATTMUX_TEST_ANDROID_LOG_H
#include <stdarg.h>
#include <stdio.h>
#define ANDROID_LOG_ERROR 6
#define ANDROID_LOG_INFO 4
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
