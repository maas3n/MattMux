/* Test a real GLX context, not just whether the dispatcher can be loaded. */
#include <stdio.h>
#include <GL/glx.h>

int main(void) {
    Display *display = XOpenDisplay(NULL);
    if (!display) { fputs("Cannot open X display\n", stderr); return 1; }
    int attrs[] = {GLX_RGBA, GLX_DOUBLEBUFFER, GLX_RED_SIZE, 8,
                   GLX_GREEN_SIZE, 8, GLX_BLUE_SIZE, 8, GLX_ALPHA_SIZE, 8,
                   GLX_DEPTH_SIZE, 24, GLX_STENCIL_SIZE, 8, None};
    XVisualInfo *visual = glXChooseVisual(display, DefaultScreen(display), attrs);
    if (!visual) { fputs("No usable GLX visual\n", stderr); return 1; }
    GLXContext context = glXCreateContext(display, visual, NULL, True);
    if (!context) { fputs("Cannot create GLX context\n", stderr); return 1; }
    XSetWindowAttributes wa = {0};
    wa.colormap = XCreateColormap(display, RootWindow(display, visual->screen), visual->visual, AllocNone);
    Window window = XCreateWindow(display, RootWindow(display, visual->screen),
        0, 0, 16, 16, 0, visual->depth, InputOutput, visual->visual, CWColormap, &wa);
    if (!glXMakeCurrent(display, window, context)) return 1;
    int major = 0;
    const char *version = (const char *)glGetString(GL_VERSION);
    if (!version || sscanf(version, "%d", &major) != 1 || major < 2) return 1;
    glClearColor(0.25f, 0.5f, 0.75f, 1.0f);
    glClear(GL_COLOR_BUFFER_BIT);
    glFinish();
    if (glGetError() != GL_NO_ERROR) return 1;
    printf("OpenGL %s; %s\n", version, glGetString(GL_RENDERER));
    glXMakeCurrent(display, None, NULL);
    glXDestroyContext(display, context);
    XDestroyWindow(display, window);
    XFreeColormap(display, wa.colormap);
    XFree(visual);
    XCloseDisplay(display);
    return 0;
}
