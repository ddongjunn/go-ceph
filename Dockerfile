# 빌드 단계
FROM golang:1.24-alpine AS builder
WORKDIR /app

# 의존성 복사 및 다운로드
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 복사
COPY . .

# 빌드
RUN CGO_ENABLED=1 GOOS=linux go build -o ceph-core-api ./cmd/api

# 실행 단계
FROM alpine:latest
WORKDIR /app

# Ceph 라이브러리 설치
RUN apk add --no-cache librados ceph-common

# 빌드된 바이너리 복사
COPY --from=builder /app/ceph-core-api .

# 포트 노출
EXPOSE 9080

# 실행
CMD ["ls -al", "./ceph-core-api"]