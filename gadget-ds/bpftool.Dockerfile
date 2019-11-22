FROM ubuntu:18.04
RUN apt-get update && \
apt-get upgrade -y && \
apt-get install -y --no-install-recommends \
    gpg gpg-agent libelf-dev libmnl-dev libc6-dev-i386 iptables libgcc-5-dev \
    bash-completion binutils binutils-dev ca-certificates make git curl \
    ca-certificates xz-utils gcc git pkg-config bison flex build-essential && \
apt-get purge --auto-remove && \
apt-get clean
WORKDIR /tmp
RUN \
git clone --depth 1 -b alban/bpftool-all https://github.com/kinvolk/linux.git && \
cd linux/tools/bpf/bpftool/ && \
sed -i '/CFLAGS += -O2/a CFLAGS += -static' Makefile && \
sed -i 's/LIBS = -lelf $(LIBBPF)/LIBS = -lelf -lz $(LIBBPF)/g' Makefile && \
printf 'feature-libbfd=0\nfeature-libelf=1\nfeature-bpf=1\nfeature-libelf-mmap=1' >> FEATURES_DUMP.bpftool && \
FEATURES_DUMP=`pwd`/FEATURES_DUMP.bpftool make -j `getconf _NPROCESSORS_ONLN` && \
strip bpftool && \
cp /tmp/linux/tools/bpf/bpftool/bpftool /bin/bpftool

