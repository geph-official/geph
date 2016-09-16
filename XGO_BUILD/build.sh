#!/bin/sh

xgo --targets=linux/386 --deps=https://github.com/jedisct1/libsodium/releases/download/1.0.11/libsodium-1.0.11.tar.gz --depsargs='CFLAGS=-m32 LDFLAGS=-m32' --out="geph-$(git describe --always)" ../
xgo -targets=darwin/386,windows/386,android/386,android/arm,linux/amd64 -deps=https://github.com/jedisct1/libsodium/releases/download/1.0.11/libsodium-1.0.11.tar.gz -out="geph-$(git describe --always)" ../
sudo chown builder *
