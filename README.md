# ceph-librados-api

`go-ceph` 기반으로 Ceph 클러스터 정보를 조회하고,  
특히 **RBD 이미지의 실제 사용 객체 기준으로 PG/OSD 매핑을 빠르게 추출**하기 위한 프로젝트입니다.

## 내가 해결한 문제

Ceph 클러스터 내 특정 RBD 이미지가 어떤 PG(Placement Group)에 매핑되고,  
그 PG가 어떤 OSD 집합으로 배치되는지 **빠르고 정확하게** 조회하는 기능

핵심 포인트:
- 관리용 외부 API 의존 대신 `go-ceph` 네이티브 접근
- 전체 객체 단순 순회 대신 `DiffIterate()` 기반의 실제 사용 블록만 분석
- 객체 → PG, PG → OSD 단계를 워커 풀로 병렬 처리

## 핵심 처리 흐름

1. Ceph 클러스터 연결 (`rados.Conn`)
2. 대상 pool/image 오픈 (`IOContext`, `rbd.Image`)
3. `DiffIterate()`로 실제 데이터가 존재하는 객체만 수집
4. 객체명 기준 `osd map` 호출로 PGID 추출
5. 고유 PGID 기준 `pg map` 호출로 OSD 목록 추출
6. 결과를 JSON 배열(`[{pg, osds}]`)로 반환

관련 코드:
- `internal/core/rbd/pg_osd_mapping.go`
  - `MapUsedObjectsToOSDs` (DiffIterate + 병렬 매핑)
  - `MapPGsToOSDs` (iterator 기반 비교용 구현)

## 성능 실험 요약

환경(로컬 컨테이너):
- Image Size: 11.8 GiB (총 공간 500 GiB)
- Block Size: 4 MB
- 총 객체 수: 128,000

주요 결과:

| 순위 | 방식 | 실행 시간 |
|---|---|---|
| 1위 | 병렬 처리 + DiffIterate (32워커) | 893.819084ms |
| 2위 | 병렬 처리 + DiffIterate (16워커) | 1.141702126초 |
| 3위 | 병렬 처리 (16워커) | 1.786228834초 |
| 4위 | 병렬 처리 (8워커) | 2.643584418초 |
| 5위 | 병렬 처리 (4워커) | 4.739434293초 |
| 6위 | 병렬 Iterator (4워커) | 14.77876159초 |
| 7위 | 병렬 Iterator (8워커) | 15.10069609초 |
| 8위 | DiffIterate만 분석 | 28.329383763초 |
| 9위 | 순차 조회 | 29.429473597초 |

즉, Sparse 이미지 환경에서 `DiffIterate()`를 사용하면 불필요한 I/O를 줄여 분석 속도를 개선할 수 있습니다.

## 현재 레포 구조

```text
.
├── cmd/api/main.go
├── internal/api
│   ├── handlers
│   ├── config
│   └── router.go
├── internal/core
│   ├── rados
│   ├── rbd
│   ├── pg
│   ├── cluster
│   ├── cephfs
│   ├── ssh
│   └── nvme
├── internal/logger
└── pkg/models
```

정리:
- `cmd/api/main.go`: API 서버 엔트리포인트
- `internal/core/rbd`: RBD PG/OSD 매핑 실험 핵심 로직
- `internal/core/rados`, `internal/core/pg`: Ceph MonCommand 기반 조회 로직
- `internal/api`: REST 핸들러/라우팅

## 사전 요구사항
- Ceph 클러스터 접근 가능 환경
- `ceph.conf` 및 인증 키링 설정
- `librados`, `librbd` 사용 가능한 시스템 라이브러리

로컬 실행 기본 포트: `:9083`

예시 API:
- `GET /api/cluster/fsid`
- `GET /api/v1/pgs`
- `GET /api/v1/pools`
- `GET /api/v1/pool/name/:pool_name/pgs`
- `GET /api/v1/pool/id/:pool_id/pgs`
- `GET /api/v1/osd/tree`
