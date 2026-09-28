#include <jni.h>
#include <MediaInfo/MediaInfo.h>
#include <ZenLib/Ztring.h>
extern "C" JNIEXPORT jbyteArray JNICALL
Java_io_github_maas3n_mattmux_MediaInfoNative_metadata(JNIEnv *env, jobject, jstring filename) {
 const char *path = env->GetStringUTFChars(filename, nullptr);
 if (!path) return nullptr;
 MediaInfoLib::MediaInfo info;
 ZenLib::Ztring input; input.From_UTF8(path);
 env->ReleaseStringUTFChars(filename, path);
 if (!info.Open(input)) {
  env->ThrowNew(env->FindClass("java/lang/IllegalStateException"), "MediaInfo could not open the MKV source");
  return nullptr;
 }
 auto result = ZenLib::Ztring(info.Inform()).To_UTF8();
 info.Close();
 jbyteArray bytes = env->NewByteArray(result.size());
 if (bytes) env->SetByteArrayRegion(bytes, 0, result.size(), reinterpret_cast<const jbyte *>(result.data()));
 return bytes;
}
