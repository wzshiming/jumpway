#!/usr/bin/env bash

IN=icon/icon_black
OUT=release/JumpWay.app/Contents/Resources/JumpWay
TMP=icons.iconset
mkdir -p "${TMP}"
LIST=(16 32 64 128 256 512 1024)
for SIZE in "${LIST[@]}"; do
  sips -z "${SIZE}" "${SIZE}" "${IN}.png" -o "${TMP}/icon_${SIZE}x${SIZE}.png"
done

iconutil -c icns "${TMP}" -o "${OUT}.icns"

rm -rf "${TMP}"