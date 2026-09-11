#include <jni.h>
#include <fcntl.h>
#include <unistd.h>
JNIEXPORT jint JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_openPath(JNIEnv *env, jclass cls, jstring path, jboolean output)
{
    (void)cls;
    const char *name = (*env)->GetStringUTFChars(env, path, NULL);
    if (!name) return -1;
    int fd = open(name, output ? O_CREAT | O_TRUNC | O_RDWR : O_RDONLY, 0600);
    (*env)->ReleaseStringUTFChars(env, path, name);
    if (fd < 0) (*env)->ThrowNew(env, (*env)->FindClass(env, "java/io/IOException"), "Could not open fixture path");
    return fd;
}
JNIEXPORT void JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_closePath(JNIEnv *env, jclass cls, jint fd)
{
    (void)env; (void)cls; close(fd);
}
