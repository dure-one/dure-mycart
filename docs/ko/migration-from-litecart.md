# litecart에서 myCart로 마이그레이션

이 가이드는 `litecart`(모든 버전)에서 `mycart`로 업그레이드할 때 필요한 모든 작업을 다룹니다.

> **데이터는 안전합니다.** 데이터베이스 스키마, 볼륨 경로(`lc_base`, `lc_digitals`, `lc_uploads`, `site`), 설정 파일은 **변경되지 않았습니다**. 프로젝트 이름, 바이너리 이름, Docker 이미지 이름, Go 모듈 경로, 저장소 URL만 업데이트되었습니다.

---

## 변경된 사항

| 이전 (litecart) | 이후 (myCart) |
|---|---|
| 바이너리: `litecart` | 바이너리: `mycart` |
| Docker 이미지: `shurco/litecart` | Docker 이미지: `shurco/mycart` |
| GHCR 이미지: `ghcr.io/shurco/litecart` | GHCR 이미지: `ghcr.io/shurco/mycart` |
| Homebrew: `brew install shurco/tap/litecart` | Homebrew: `brew install shurco/tap/mycart` |
| Go 모듈: `github.com/shurco/litecart` | Go 모듈: `github.com/shurco/mycart` |
| 저장소: `github.com/shurco/litecart` | 저장소: `github.com/shurco/mycart` |
| CLI 명령: `./litecart serve` | CLI 명령: `./mycart serve` |
| 컨테이너 이름: `litecart` | 컨테이너 이름: `mycart` |

## 변경되지 않은 사항

- 데이터베이스 형식 및 스키마 — 완전히 호환되며 마이그레이션 불필요
- 볼륨 마운트 경로: `./lc_base`, `./lc_digitals`, `./lc_uploads`, `./site`
- API 엔드포인트 및 응답 형식
- 관리자 패널 URL (`/_/`)
- 데이터베이스에 저장된 설정
- 기본 포트 (`8080`)

---

## 바이너리 (Linux / macOS / Windows)

### 1단계 — 백업

```bash
cp -r ./lc_base ./lc_base_backup
cp -r ./site ./site_backup
```

### 2단계 — 새 바이너리 다운로드

```bash
curl -L https://raw.githubusercontent.com/shurco/mycart/main/scripts/install | sh
```

또는 [Releases](https://github.com/shurco/mycart/releases/latest)에서 수동으로 다운로드하세요.

### 3단계 — 이전 바이너리 제거

```bash
rm ./litecart
```

### 4단계 — 실행

```bash
./mycart serve
```

새 바이너리는 기존 `./lc_base/data.db` 데이터베이스와 모든 볼륨을 자동으로 인식합니다.

---

## Homebrew (macOS)

```bash
brew uninstall litecart
brew untap shurco/tap 2>/dev/null
brew tap shurco/tap
brew install mycart
```

---

## Docker

### 1단계 — 이전 컨테이너 중지

```bash
docker stop litecart
```

### 2단계 — 새 이미지 가져오기

```bash
# Docker Hub
docker pull shurco/mycart:latest

# or GitHub Container Registry
docker pull ghcr.io/shurco/mycart:latest
```

### 3단계 — 이전 컨테이너 이름 변경 (백업)

```bash
docker rename litecart litecart-backup
```

### 4단계 — 새 컨테이너 시작

**동일한 볼륨 마운트**를 사용하세요 — 데이터는 완전히 호환됩니다:

```bash
docker run \
  --name mycart \
  --restart unless-stopped \
  -p '8080:8080' \
  -v ./lc_base:/lc_base \
  -v ./lc_digitals:/lc_digitals \
  -v ./lc_uploads:/lc_uploads \
  -v ./site:/site \
  shurco/mycart:latest
```

### 5단계 — 확인 및 정리

모든 것이 정상 작동하면 이전 컨테이너를 제거하세요:

```bash
docker rm litecart-backup
docker rmi shurco/litecart:latest
```

---

## Docker Compose

### 1단계 — `docker-compose.yml` 업데이트

서비스 정의를 교체합니다:

```yaml
# Before
services:
  litecart:
    image: shurco/litecart:latest
    container_name: litecart

# After
services:
  mycart:
    image: shurco/mycart:latest
    container_name: mycart
```

### 2단계 — nginx 설정 업데이트

번들된 nginx 리버스 프록시를 사용하는 경우 `docker/nginx/nginx.conf`를 업데이트하세요:

```nginx
# Before
upstream cart {
    server litecart:8080;
}

# After
upstream cart {
    server mycart:8080;
}
```

### 3단계 — 재시작

```bash
docker-compose down
docker-compose up -d
```

---

## Kubernetes

### 1단계 — 매니페스트 업데이트

K8s 매니페스트의 모든 항목을 교체합니다:

| 리소스 | 이전 이름 | 새 이름 |
|---|---|---|
| PVC | `litecart-pvc` | `mycart-pvc` |
| Deployment | `litecart` | `mycart` |
| 컨테이너 이미지 | `shurco/litecart:latest` | `shurco/mycart:latest` |
| 컨테이너 이름 | `litecart` | `mycart` |
| 볼륨 이름 | `litecart-storage` | `mycart-storage` |
| Service | `litecart-service` | `mycart-service` |
| Ingress | `litecart-ingress` | `mycart-ingress` |
| TLS secret | `litecart-tls` | `mycart-tls` |
| 라벨 | `app: litecart` | `app: mycart` |

업데이트된 매니페스트 예제는 [`/k8s/`](../k8s/)에서 확인할 수 있습니다.

### 2단계 — 적용

```bash
kubectl apply -f k8s/mycart.yaml
```

> **참고:** `ReadWriteOnce` PVC를 사용하는 경우, 볼륨을 해제하기 위해 새 배포를 만들기 전에 기존 배포를 먼저 삭제해야 할 수 있습니다.

---

## Go 모듈 (패키지를 가져오는 개발자용)

Go 프로젝트에서 `litecart` 패키지를 가져오는 경우:

```go
// Before
import "github.com/shurco/litecart/pkg/litepay"

// After
import "github.com/shurco/mycart/pkg/litepay"
```

`go.mod`를 업데이트하세요:

```bash
go get github.com/shurco/mycart@latest
```

그런 다음 이전 의존성을 제거합니다:

```bash
go mod tidy
```

---

## CI/CD 파이프라인

CI/CD 설정의 모든 참조를 업데이트하세요:

- Docker 이미지 참조: `shurco/litecart` → `shurco/mycart`
- 바이너리 다운로드 URL: `github.com/shurco/litecart/releases` → `github.com/shurco/mycart/releases`
- 설치 스크립트 URL: `raw.githubusercontent.com/shurco/litecart/main/scripts/install` → `raw.githubusercontent.com/shurco/mycart/main/scripts/install`
- Go 모듈 경로: `github.com/shurco/litecart` → `github.com/shurco/mycart`

---

## 문제 해결

**Q: 기존 데이터베이스가 새 버전에서도 작동하나요?**
A: 예. 데이터베이스 스키마는 변경되지 않았습니다. 새 바이너리는 동일한 `./lc_base/data.db` 파일을 읽습니다.

**Q: 결제 시스템을 다시 설정해야 하나요?**
A: 아니요. 모든 설정은 데이터베이스에 저장되어 있으며 그대로 유지됩니다.

**Q: 기존 저장소는 계속 접근 가능한가요?**
A: `github.com/shurco/litecart`의 기존 저장소는 전환 기간 동안 새 위치로 리디렉션됩니다.

**Q: 자체 업데이트 기능(`./litecart update`)을 사용하고 있습니다. 계속 작동하나요?**
A: 아니요. 기존 바이너리는 `litecart` 저장소 이름으로 릴리스를 찾습니다. 위 안내에 따라 `mycart`를 수동으로 다운로드하면, 이후 `./mycart update`를 통한 업데이트는 정상적으로 작동합니다.
