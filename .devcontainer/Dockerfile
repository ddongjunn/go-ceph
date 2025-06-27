FROM ubuntu:22.04

# 필수 패키지 설치
RUN apt update && apt install -y \
    gnupg curl ca-certificates lsb-release

# Ceph Reef APT 저장소 등록
RUN curl -fsSL https://download.ceph.com/keys/release.asc | gpg --dearmor -o /etc/apt/trusted.gpg.d/ceph.gpg && \
    echo "deb https://download.ceph.com/debian-reef/ $(lsb_release -sc) main" > /etc/apt/sources.list.d/ceph.list

# Ceph Reef 및 개발 헤더 설치
RUN apt update && apt install -y \
    ceph-common librados-dev librbd-dev libcephfs-dev \
    build-essential git curl vim openssh-server

# Go 설치 (ARM64)
RUN curl -LO https://go.dev/dl/go1.21.9.linux-arm64.tar.gz && \
    tar -C /usr/local -xzf go1.21.9.linux-arm64.tar.gz && \
    rm go1.21.9.linux-arm64.tar.gz

ENV PATH="/usr/local/go/bin:$PATH"
ENV GO111MODULE=on

WORKDIR /app