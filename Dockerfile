# 빌드 단계
FROM --platform=linux/amd64 golang:1.24-bookworm AS builder
WORKDIR /app
LABEL ceph="true"

RUN apt-get update && apt-get install -y \
  build-essential \
  librados-dev \
  librbd-dev \
  ceph-common \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags no_encryption -o ceph-core-api ./cmd/api

# 실행 단계
FROM --platform=linux/amd64 debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y librados2 ceph-common && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/ceph-core-api .

EXPOSE 9080
CMD ["./ceph-core-api"]