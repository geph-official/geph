#!/bin/sh

xgo -targets=linux/amd64 -deps=https://github.com/jedisct1/libsodium/releases/download/1.0.11/libsodium-1.0.11.tar.gz -depsargs='CFLAGS=-Os' -out="geph-$(git describe --always)" -ldflags='-w' ../
#sudo chown builder *
ln -sf "geph-$(git describe --always)-linux-amd64" "geph-latest-linux-amd64"
sudo chown builder *
