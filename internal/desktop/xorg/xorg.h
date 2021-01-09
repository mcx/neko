#pragma once

#ifndef XDISPLAY_H
#define XDISPLAY_H

#include <X11/Xlib.h>
#include <X11/XKBlib.h>
#include <X11/extensions/Xrandr.h>
#include <X11/extensions/XTest.h>
#include <X11/extensions/Xfixes.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

extern void goCreateScreenSize(int index, int width, int height, int mwidth, int mheight);
extern void goSetScreenRates(int index, int rate_index, short rate);

Display *getXDisplay(void);
int XDisplayOpen(char *input);
void XDisplayClose(void);

void XMove(int x, int y);
void XScroll(int x, int y);
void XButton(unsigned int button, int down);
void XKey(unsigned long key, int down);

void XGetScreenConfigurations();
void XSetScreenConfiguration(int index, short rate);
int XGetScreenSize();
short XGetScreenRate();

void SetKeyboardLayout(char *layout);
void SetKeyboardModifiers(int num_lock, int caps_lock, int scroll_lock);
XFixesCursorImage *XGetCursorImage(void);

#endif
