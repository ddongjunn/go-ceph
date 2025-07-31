# === 빌드 단계 ===
FROM --platform=linux/amd64 golang:1.24-bookworm AS builder
WORKDIR /app

# Ceph 라이브러리만 설치 (Reef repo 사용하지 않아도 됨)
RUN apt-get update && apt-get install -y \
    build-essential \
    librados-dev \
    librbd-dev \
    ceph-common \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Reef 기준 encryption 관련 문제 회피 (v0.34.0 사용시)
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags no_encryption -o ceph-core-api ./cmd/api

# === 실행 단계 ===
FROM --platform=linux/amd64 debian:bookworm-slim
WORKDIR /app

# 런타임에 필요한 라이브러리만
RUN apt-get update && apt-get install -y \
    librados2 \
    ceph-common \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/ceph-core-api .

EXPOSE 9080
CMD ["./ceph-core-api"]