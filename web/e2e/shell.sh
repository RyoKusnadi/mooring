#!/bin/sh
# Fixed shell for browser tests: plain prompt, no user rc files.
exec env PS1="\$ " HISTFILE= /bin/sh
