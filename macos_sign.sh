#!/usr/bin/env zsh

[ -n "$1" ] || { echo "invalid binary path"; exit 1; }
[ -n "$AC_APPID" ] || { echo "invalid AC_APPID"; exit 1; }

xcrun codesign -s $AC_APPID -f -v --timestamp --options runtime $1
