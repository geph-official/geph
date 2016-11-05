#!/bin/sh

xgo --targets=linux/386,windows/386 --deps=https://github.com/jedisct1/libsodium/releases/download/1.0.11/libsodium-1.0.11.tar.gz --depsargs='CFLAGS=-m32 LDFLAGS=-m32' --out="geph-$(git describe --always)" ../
xgo -targets=android/386,android/arm,linux/amd64 -deps=https://github.com/jedisct1/libsodium/releases/download/1.0.11/libsodium-1.0.11.tar.gz -out="geph-$(git describe --always)" ../
sudo chown builder *
ln -sf "geph-$(git describe --always)-linux-amd64" "geph-latest-linux-amd64"
ln -sf "geph-$(git describe --always)-android-16-arm" "geph-latest-android-16-arm"
ln -sf "geph-$(git describe --always)-android-16-386" "geph-latest-android-16-386"
ln -sf "geph-$(git describe --always)-windows-4.0-386.exe" "geph-latest-windows-4.0-386.exe"
