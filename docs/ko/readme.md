<p align="center">
    <a href="#" target="_blank" rel="noopener">
        <img src="https://github.com/shurco/mycart/blob/main/.github/media/banner.png" alt="myCart - 하나의 파일로 된 쇼핑카트" />
    </a>
</p>


<a href="https://github.com/shurco/mycart/releases"><img src="https://img.shields.io/github/v/release/shurco/mycart?sort=semver&label=Release&color=651FFF"></a>
<a href="https://goreportcard.com/report/github.com/shurco/mycart"><img src="https://goreportcard.com/badge/github.com/shurco/mycart"></a>
<a href="https://www.codefactor.io/repository/github/shurco/mycart"><img src="https://www.codefactor.io/repository/github/shurco/mycart/badge" alt="CodeFactor" /></a>
<a href="https://github.com/shurco/mycart/actions/workflows/release.yml"><img src="https://github.com/shurco/mycart/actions/workflows/release.yml/badge.svg"></a>
<a href="https://github.com/shurco/mycart/blob/master/LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg"></a>

> [!Important]
> 이 저장소는 이름이 변경되고 있습니다.  
>이 프로젝트는 원래 litecart라는 이름으로 게시되었습니다. 최근 이 이름 사용과 관련된 상표권 주장을 받았습니다. 혼란과 잠재적인 법적 문제를 피하기 위해 프로젝트는 새로운 이름으로 계속됩니다.  
>코드베이스 자체는 변경되지 않습니다. 프로젝트 이름, 저장소 이름, 패키지 식별자 및 관련 참조만 업데이트됩니다.  
>이 저장소는 기존 사용자가 마이그레이션할 시간을 갖도록 리디렉션 및 메모와 함께 일정 기간 동안 사용 가능한 상태로 유지됩니다.  
>현재 저장소 이름에 의존하는 설정이 있는 경우 이름 변경이 완료되면 참조를 업데이트하세요.  
>프로젝트를 사용하고 피드백을 제공해 주신 모든 분들께 감사드립니다.  

> [!NOTE]
> **면책 조항:** 유사한 이름의 프로젝트나 브랜드와 관련이 없습니다.
> 이것은 MIT 라이선스에 따라 라이선스가 부여된 독립적인 오픈 소스 프로젝트입니다.


## 🛒&nbsp;&nbsp;myCart란?

myCart는 임베디드 데이터베이스(SQLite) 1개 파일, 편리한 대시보드 UI 및 간단한 사이트로 구성된 오픈 소스 쇼핑카트입니다.
이전에는 **litecart**로 알려졌습니다(검색 시 발견 가능성을 위해 레거시 프로젝트 이름이 여기에 유지됨).

> [!WARNING]
> 현재 메이저 버전은 0(`v0.x.x`)이며, 사용자로부터 조기 피드백을 받는 동안 빠른 개발과 반복을 수용합니다. myCart는 여전히 활발히 개발 중이므로 v1.0.0에 도달하기 전까지는 완전한 하위 호환성이 보장되지 않습니다.

### 비디오 예제
![예제](https://github.com/shurco/mycart/blob/main/.github/media/demo.gif)

### 관리자 패널 스크린샷
<p align="center">
  <img src="https://github.com/shurco/mycart/blob/main/.github/media/screenshots/products.png" width="270">
  <img src="https://github.com/shurco/mycart/blob/main/.github/media/screenshots/product-edit.png" width="270">
  <img src="https://github.com/shurco/mycart/blob/main/.github/media/screenshots/carts.png" width="270">
  <img src="https://github.com/shurco/mycart/blob/main/.github/media/screenshots/pages.png" width="270">
  <img src="https://github.com/shurco/mycart/blob/main/.github/media/screenshots/settings.png" width="270">
</p>


## 🏆&nbsp;&nbsp;주요 기능

🚀 **간단하고 빠름**: 스토어를 빠르게 시작하고 실행할 수 있는 원클릭 설치 프로세스를 즐기세요. 시간과 노력을 절약합니다.  

💰 **인기 결제 시스템 지원**: 인기 있는 결제 시스템을 지원하여 원활하게 결제를 수락하고 고객에게 원활한 결제 경험을 제공합니다.  

🔑 **파일 및 라이선스 키 판매**: 디지털 파일이나 라이선스 키를 판매하든 myCart가 지원하므로 제공할 수 있는 제품 유형에 유연성을 제공합니다.  

⚙️ **경량 및 효율적**: myCart는 SQLite를 임베디드 데이터베이스로 사용하여 MySQL, PostgreSQL 또는 MongoDB와 같은 무거운 데이터베이스가 필요하지 않습니다. 이로 인해 탁월한 성능을 발휘하는 경량 웹사이트가 생성됩니다.  

☁️ **쉽게 사용자 정의 가능**: 브랜딩 및 고유한 요구 사항에 맞게 myCart 웹사이트를 손쉽게 수정하고 사용자 정의하여 진정으로 자신만의 것으로 만드세요.  

🧞‍♂️ **편리한 관리 패널**: 사용자 친화적인 대시보드 UI로 myCart는 번거로움 없는 관리 패널을 제공하여 스토어, 재고 및 주문을 쉽게 관리할 수 있습니다.  

⚡️ **하드웨어 호환성**: 강력한 서버나 적당한 하드웨어 설정에서 myCart를 실행하든 원활하게 작동하여 고객에게 일관된 쇼핑 경험을 제공합니다.  

🔒 **내장 HTTPS 지원**: 보안을 우선시하는 myCart는 HTTPS에 대한 기본 지원과 함께 제공되어 고객 데이터의 안전을 보장합니다.

🆓 **무료 제품 지원**: 제품 가격을 0으로 설정하여 고객에게 무료 제품을 제공합니다. 무료 제품은 결제 시스템 통합 없이 자동으로 처리되므로 무료 다운로드, 샘플 또는 프로모션 콘텐츠에 적합합니다.

🌐 **다국어 지원**: 기본 제공 국제화(i18n) 지원을 통해 다국어 스토어를 만들 수 있습니다. 기본적으로 myCart는 영어와 중국어를 지원합니다. 언어 전환기는 관리자 패널과 공개 사이트 모두에서 사용할 수 있으므로 여러 언어로 콘텐츠를 쉽게 관리하고 고객에게 현지화된 쇼핑 경험을 제공할 수 있습니다.

🎨 **제품 변형**: 각 변형에 대해 별도의 재고 추적, 가격 및 SKU를 사용하여 여러 옵션(크기, 색상, 스타일 등)이 있는 제품을 제공합니다. 모든 조합을 자동으로 생성하거나 수량 및 가용성 제어를 사용하여 특정 변형을 수동으로 관리합니다.


## ⬇️&nbsp;&nbsp;설치

`mycart`는 터미널에서 단일 명령만 필요한 쉬운 설치 및 작동을 위해 설계되었습니다. 기존 설치 방법 외에도 `mycart`는 HomeBrew, Docker 또는 Docker Compose, Docker Swarm, Rancher 또는 Kubernetes와 같은 기타 컨테이너 오케스트레이션 도구를 통해 설정 및 작동할 수 있습니다.

#### <img width="20" src="https://raw.githubusercontent.com/shurco/mycart/main/.github/media/platforms/apple.svg">&nbsp;macOS에 설치
macOS에 `mycart`를 설치하는 가장 빠른 방법은 Homebrew를 사용하는 것입니다. 이렇게 하면 명령줄 도구와 `mycart` 서버가 결합된 실행 파일로 설치됩니다. Homebrew를 사용하지 않는 경우 아래 Linux 지침에 따라 `mycart`를 설치하세요.
```shell
brew install shurco/tap/mycart
```

또는 탭을 구성하고 패키지를 별도로 설치할 수 있습니다:
``` shell
$ brew tap shurco/tap
$ brew install mycart
```


#### <img width="20" src="https://raw.githubusercontent.com/shurco/mycart/main/.github/media/platforms/linux.svg">&nbsp;Linux에 설치 
Unix 운영 체제에서 `mycart` 사용을 시작하는 가장 간단하고 권장되는 방법은 `mycart` 명령줄 도구를 설치하고 사용하는 것입니다. 터미널에서 다음 명령을 실행하고 화면에 표시된 지침을 따르세요.

```bash
curl -L https://raw.githubusercontent.com/shurco/mycart/main/scripts/install | sh
```

#### <img width="20" src="https://raw.githubusercontent.com/shurco/mycart/main/.github/media/platforms/windows.svg">&nbsp;Windows에 설치
Windows에서 `mycart` 사용을 시작하는 가장 간단하고 권장되는 방법은 `mycart` 명령줄 도구를 설치하고 사용하는 것입니다. 터미널에서 다음 명령을 실행하고 화면에 표시된 지침을 따르세요.
```bash
curl -L https://raw.githubusercontent.com/shurco/mycart/main/scripts/install | sh
```
또는 Windows용 [최신 버전](https://github.com/shurco/mycart/releases/latest)을 다운로드하여 압축을 풉니다.


#### <img width="20" src="https://raw.githubusercontent.com/shurco/mycart/main/.github/media/platforms/docker.svg">&nbsp;Docker를 사용하여 실행
Docker를 사용하면 명령줄 도구를 설치할 필요 없이 `mycart` 인스턴스를 관리하고 작동할 수 있습니다. `mycart` Docker 컨테이너에는 필요한 모든 명령줄 도구 또는 서버 실행이 포함되어 있습니다.

[Docker Hub](https://hub.docker.com/r/shurco/mycart)의 경우:
```bash
docker run \
  -v ./lc_base:/lc_base \
  -v ./lc_digitals:/lc_digitals \
  -v ./lc_uploads:/lc_uploads \
  -v ./site:/site \
  --rm shurco/mycart:latest init

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
또는 [Github Packages Hub](https://github.com/shurco/mycart/pkgs/container/mycart)를 사용하는 경우:

```bash
docker run \
  -v ./lc_base:/lc_base \
  -v ./lc_digitals:/lc_digitals \
  -v ./lc_uploads:/lc_uploads \
  -v ./site:/site \
  --rm ghcr.io/shurco/mycart:latest init

docker run \
  --name mycart \
  --restart unless-stopped \
  -p '8080:8080' \
  -v ./lc_base:/lc_base \
  -v ./lc_digitals:/lc_digitals \
  -v ./lc_uploads:/lc_uploads \
  -v ./site:/site \
  ghcr.io/shurco/mycart:latest
```

#### <img width="20" src="https://raw.githubusercontent.com/shurco/mycart/main/.github/media/platforms/docker.svg">&nbsp;Docker Compose를 사용하여 실행
Docker Compose는 여러 컨테이너와 서비스를 관리하는 편리한 방법을 제공합니다. 프로젝트에는 다양한 사용 사례를 위한 여러 Docker Compose 구성이 포함되어 있습니다.

**프로덕션 설정** (`docker/docker-compose.yml`):
이 구성에는 myCart 애플리케이션과 Nginx 리버스 프록시가 포함됩니다:

```bash
cd docker
docker-compose up -d
```

이 설정에는 다음이 포함됩니다:
- **mycart**: 필요한 모든 볼륨이 있는 메인 애플리케이션 컨테이너
- **nginx**: 포트 80에서 수신 대기하고 요청을 myCart로 전달하는 리버스 프록시 서버

Nginx 구성은 `docker/nginx/nginx.conf`에 있으며 필요에 따라 사용자 정의할 수 있습니다.

**개발 설정** (`docker/docker-compose_dev.yml`):
로컬 개발의 경우 이메일 테스트를 위해 MailHog를 포함하는 개발 구성을 사용할 수 있습니다:

```bash
cd docker
docker-compose -f docker-compose.yml -f docker-compose_dev.yml up -d
```

다음이 추가됩니다:
- **mailhog**: `http://localhost:8025`에서 액세스할 수 있는 이메일 테스트 도구로 전송된 이메일을 볼 수 있습니다

**결합 사용**:
프로덕션 및 개발 서비스를 함께 실행하려면:

```bash
cd docker
docker-compose -f docker-compose.yml -f docker-compose_dev.yml up -d
```

**초기화**:
서비스를 시작하기 전에 애플리케이션을 초기화합니다:

```bash
docker-compose run --rm mycart init
```

**최초 관리자 계정** (하나를 선택):

1. 브라우저에서 `http://localhost/_/install`을 열고 설정 마법사를 완료합니다.
2. 또는 비대화형으로 관리자 계정을 만듭니다(셸이 없는 스크래치 기반 이미지에서 작동):

```bash
docker-compose run --rm mycart install \
  --email admin@example.com \
  --password 'YourSecurePass' \
  --domain localhost
```

Kubernetes의 경우 메인 Deployment와 동일한 볼륨 마운트로 동일한 `install` 명령을 사용하여 일회성 Job을 실행합니다.

**환경 변수**:
docker-compose 설정은 도메인 및 이메일 구성을 위한 환경 변수를 지원합니다:

```bash
DOMAIN=example.com ADMIN_EMAIL=admin@example.com docker-compose -f docker/docker-compose.yml up -d
```

또는 `docker/` 디렉터리에 `.env` 파일을 만듭니다:
```bash
DOMAIN=example.com
ADMIN_EMAIL=admin@example.com
```

**서비스 중지**:
```bash
docker-compose down
```

#### <img width="20" src="https://raw.githubusercontent.com/shurco/mycart/main/.github/media/platforms/k8s.svg">&nbsp;Kubernetes를 사용하여 실행
Kubernetes에서 실행하기 위한 예제 매니페스트는 `/k8s/` 폴더에서 찾을 수 있습니다(<a href="https://github.com/vuisme" target="_blank">@vuisme</a>님 감사합니다)


## 🔄&nbsp;&nbsp;litecart에서 마이그레이션

이전 이름 **litecart**로 게시된 버전에서 업그레이드하는 경우 바이너리, Docker, Docker Compose, Kubernetes, Homebrew 및 Go 모듈 업데이트를 다루는 단계별 지침은 **[마이그레이션 가이드](/migration-from-litecart)**를 참조하세요. 데이터와 데이터베이스는 완전히 호환됩니다. 스키마 마이그레이션이 필요하지 않습니다.

## ⬇️&nbsp;&nbsp;업데이트
> [!WARNING]
> 업데이트하기 전에 *./lc_base* 폴더와 *./site* 폴더를 백업하세요.

#### macOS / Linux / Windows에서 업데이트
`mycart`를 최신 버전으로 업데이트하는 가장 쉬운 방법은 다음 명령을 실행하는 것입니다:

```bash
./mycart update
```

업데이트 중에 데이터베이스 구조에 변경 사항이 있는 경우 마이그레이션을 수행해야 합니다. 이를 위해 `mycart` 폴더에서 다음 명령을 실행해야 합니다:
```bash
./mycart migrate
```


#### <img width="20" src="https://raw.githubusercontent.com/shurco/mycart/main/.github/media/platforms/docker.svg">&nbsp; Docker를 사용하여 업데이트
우리의 만트라는 업데이트를 원활한 경험으로 만드는 것입니다. 새 이미지를 다운로드하고 평소처럼 컨테이너를 시작하기만 하면 됩니다. 예를 들어 [Docker Hub](https://hub.docker.com/r/shurco/mycart)를 사용하는 경우:

```bash
docker stop mycart
docker pull shurco/mycart:latest # 새 이미지 다운로드
docker rename mycart mycart-backup # 이미지 백업
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

업데이트 중에 데이터베이스 구조에 변경 사항이 있는 경우 마이그레이션을 수행해야 합니다. 이를 위해 `mycart` 폴더에서 다음 명령을 실행해야 합니다:
```bash
docker run \
-v ./lc_base:/lc_base \
-v ./site:/site \
--rm shurco/mycart migrate
```

#### <img width="20" src="https://raw.githubusercontent.com/shurco/mycart/main/.github/media/platforms/k8s.svg">&nbsp;Kubernetes를 사용하여 실행
Kubernetes에서 실행하기 위한 예제 매니페스트는 `/k8s/` 폴더에서 찾을 수 있습니다(<a href="https://github.com/vuisme" target="_blank">@vuisme</a>님 감사합니다)

## 🚀&nbsp;&nbsp;시작하기
`mycart`를 시작하는 것은 `mycart` 서버를 시작하는 것만큼 쉽습니다

Linux/macOS의 기본 실행:
```bash
./mycart serve
```

Windows의 경우:
```
mycart.exe serve
```

처음 실행할 때 실행 파일이 있는 디렉터리에 필요한 폴더가 생성됩니다. 액세스를 위한 기본 링크는 다음과 같습니다:  
- [http://localhost:8080](http://localhost:8080) - 웹사이트  
- [http://localhost:8080/_/](http://localhost:8080/_/) - 제어판  

다른 포트에서 실행해야 하는 경우 `--http` 플래그를 사용하세요:
```
./mycart serve --http 0.0.0.0:8088
```

> [!NOTE]
> 포트 <= 1024는 권한이 있는 포트입니다. root가 아니거나 명시적 권한이 없으면 사용할 수 없습니다. 설명은 이 답변 또는 위키백과 또는 더 신뢰할 수 있는 것을 참조하세요. 사용:
> **sudo setcap 'cap_net_bind_service=+ep' /path_to/mycart**

## 📚&nbsp;&nbsp;명령어
사용법:
```
./mycart [command] [flags]
```

사용 가능한 명령어:
```
init        기본 구조 생성
migrate     최신 버전의 데이터베이스 스키마로 마이그레이션
serve       웹 서버 시작(기본값 0.0.0.0:8080)
update      애플리케이션을 최신 버전으로 업데이트
```

전역 플래그 `./mycart [flags]`:
```
-h, --help      mycart에 대한 도움말
-v, --version   mycart 버전
```

Serve 플래그 `./mycart serve [flags]`:
```
--http string    서버 주소(기본값 "0.0.0.0:8080")
--https string   https 서버 주소(자동 TLS)
--no-site        사이트 생성 비활성화
```

## 🏦&nbsp;&nbsp;결제 시스템 추가
#### Stripe
Stripe는 고객으로부터 온라인 결제를 수락할 수 있는 인기 있는 결제 시스템입니다. 신용 카드 및 직불 카드, 디지털 지갑 및 은행 이체를 수락하는 기능을 포함하여 결제 처리를 위한 다양한 도구 및 API를 제공합니다. Stripe는 결제 보안, 통화 처리 및 다양한 결제 방법 지원을 보장합니다.

Stripe에서 Secret key를 얻으려면 다음 단계를 따르세요:

1. 공식 Stripe 웹사이트에서 <a href="https://dashboard.stripe.com" target="_blank">Stripe 계정</a>에 로그인합니다. 계정이 없으면 <a href="https://dashboard.stripe.com/register" target="_blank">등록</a>하세요.
2. 오른쪽 상단에서 <a href="https://dashboard.stripe.com/developers" target="_blank">개발자 섹션</a>을 선택합니다.
3. 드롭다운 메뉴에서 "<a href="https://dashboard.stripe.com/apikeys" target="_blank">API Keys</a>"를 선택합니다.
4. "Standard keys" 섹션에서 "Secret key"를 찾을 수 있습니다.

> [!WARNING]
> "Secret key"는 안전하게 보관해야 하는 기밀 정보입니다.


#### PayPal
PayPal은 개인과 기업이 인터넷을 통해 돈을 보내고 받을 수 있는 온라인 결제 시스템입니다. 상품 및 서비스에 대한 결제와 사용자 간 이체를 가능하게 합니다. PayPal은 전자 결제를 안전하고 편리하게 만드는 방법을 제공합니다.

PayPal API를 사용하기 위한 Client ID 및 Secret Key를 얻으려면 다음 단계를 따르세요:

1. API를 사용하려면 PayPal 비즈니스 계정이 필요합니다.
2. <a href="https://developer.paypal.com/" target="_blank">PayPal Developer</a> 웹사이트로 이동하여 PayPal 비즈니스 계정 자격 증명으로 로그인합니다.
3. 대시보드에서 "My Apps & Credentials" 섹션을 찾고 "Create App" 버튼을 클릭하여 새 애플리케이션을 만듭니다.
4. 애플리케이션 페이지에서 Client ID를 볼 수 있습니다. 애플리케이션을 만든 직후에 표시됩니다. Secret Key를 보려면 "Secret" 레이블 아래의 "Show" 버튼을 클릭합니다.

> [!WARNING]
> "Secret key"는 안전하게 보관해야 하는 기밀 정보입니다.


#### SpectroCoin
<a href="https://spectrocoin.com/en/invite?referralId=b2n87748" target="_blank">SpectroCoin</a>은 사용자가 Bitcoin, Ethereum 등 다양한 통화로 결제를 보내고 받을 수 있는 결제 시스템 및 암호화폐 지갑입니다. 또한 다양한 통화 간 통화 교환 작업 및 은행 계좌로 자금을 입금하고 인출하는 기능을 제공합니다. <a href="https://spectrocoin.com/en/invite?referralId=b2n87748" target="_blank">SpectroCoin</a>은 결제 및 암호화폐 저장의 보안을 보장하며 직불 카드와 같은 추가 기능을 제공합니다.

<a href="https://spectrocoin.com/en/invite?referralId=b2n87748" target="_blank">SpectroCoin</a>에서 "Merchant ID", "Project (API) ID" 및 "Private key"를 얻으려면 다음 단계를 따르세요:

1. 아직 계정이 없으면 <a href="https://spectrocoin.com/en/invite?referralId=b2n87748" target="_blank">SpectroCoin</a>에 등록합니다.
2. <a href="https://spectrocoin.com/en/invite?referralId=b2n87748" target="_blank">SpectroCoin</a> 계정에 로그인합니다.
3. 탐색 메뉴에서 "Business" 섹션으로 이동합니다.
4. 탐색 메뉴에서 "New project" 섹션으로 이동합니다.
5. 프로젝트 이름을 입력하고 "Public key" 섹션을 활성화하세요. "Private key"가 있는 창이 나타나면 복사하여 저장합니다. 필요한 경우 다른 옵션을 활성화할 수 있습니다.
6. 세부 정보를 입력한 후 프로젝트 페이지로 리디렉션됩니다. 생성된 프로젝트로 이동하여 헤더에서 "Merchant ID" 및 "Project (API) ID"를 복사합니다.

> [!WARNING]
> 프로젝트를 만들려면 <a href="https://spectrocoin.com/en/invite?referralId=b2n87748" target="_blank">SpectroCoin</a> 계정에 대한 확인 프로세스를 완료해야 할 수 있습니다.  
> "Private key"는 안전하게 보관해야 하는 기밀 정보입니다.


#### Coinbase
Coinbase Commerce는 기업이 Bitcoin, Ethereum, Litecoin 등 다양한 암호화폐로 결제를 수락할 수 있는 암호화폐 결제 플랫폼입니다. 스토어에 암호화 결제를 통합하는 간단하고 안전한 방법을 제공합니다.

Coinbase Commerce API를 사용하기 위한 API Key를 얻으려면 다음 단계를 따르세요:

1. <a href="https://commerce.coinbase.com" target="_blank">Coinbase Commerce</a> 계정을 만들거나 로그인합니다.
2. **Settings** 섹션으로 이동합니다.
3. **API Keys** 탭에서 **Create an API Key**를 클릭합니다.
4. 생성된 API Key를 복사하여 안전하게 저장합니다.

> [!WARNING]
> "API Key"는 안전하게 보관해야 하는 기밀 정보입니다.


#### Dummy Payment
Dummy Payment는 myCart와 함께 사전 구성되어 제공되는 기본 제공 결제 공급자입니다. 무료 제품(가격이 $0인 제품)을 처리하도록 설계되었으며 외부 결제 시스템 통합 또는 API 키가 필요하지 않습니다.

**작동 방식:**
- 고객의 카트에 무료 제품만 포함된 경우 자동으로 활성화됩니다(총 금액 = $0)
- 결제 처리가 발생하지 않습니다 - 주문이 즉시 결제됨으로 표시됩니다
- 고객은 결제를 완료하기 위해 이메일 주소만 제공하면 됩니다
- 무료 다운로드, 샘플, 프로모션 콘텐츠 및 리드 생성에 적합합니다

**주요 기능:**
- **구성 필요 없음**: 바로 사용할 수 있으며 설정이 필요하지 않습니다
- **API 키 필요 없음**: 다른 결제 공급자와 달리 Dummy Payment는 외부 계정이나 자격 증명이 필요하지 않습니다
- **즉시 처리**: 결제 확인을 기다리지 않고 주문이 즉시 처리됩니다
- **전체 기능 지원**: 이메일 전달, 디지털 파일 다운로드, 라이선스 키 및 웹훅을 포함한 모든 표준 기능이 Dummy Payment와 함께 작동합니다

**사용 시기:**
- 무료 제품 또는 샘플 판매
- 프로모션 콘텐츠 제공
- 리드 생성을 위한 이메일 주소 수집
- 개발 중 결제 프로세스 테스트

> [!NOTE]
> Dummy Payment는 무료 제품만 포함된 카트에만 사용됩니다. 카트에 무료 제품과 유료 제품이 모두 포함되어 있는 경우 고객은 구매를 완료하기 위해 일반 결제 시스템(Stripe, PayPal 또는 SpectroCoin)을 사용해야 합니다.


## 🆓&nbsp;&nbsp;무료 제품

myCart는 무료 제품을 지원하므로 고객에게 무료로 디지털 콘텐츠, 샘플 또는 프로모션 자료를 제공할 수 있습니다.

### 무료 제품 만들기

무료 제품을 만들려면:

1. 관리자 패널에서 **Products**로 이동하여 새 제품을 만듭니다
2. **Amount** 필드를 `0`(0)으로 설정합니다
3. 제품이 관리자 패널과 웹사이트 모두에서 자동으로 "무료"로 표시됩니다

### 무료 제품 작동 방식

- **자동 처리**: 고객이 카트에 무료 제품만 추가하면(총 금액 = 0) 결제 프로세스에서 기본 제공 **Dummy Payment** 공급자를 자동으로 사용합니다(자세한 내용은 [Dummy Payment](#dummy-payment) 섹션 참조)
- **결제 필요 없음**: 무료 제품은 모든 외부 결제 시스템 통합을 우회합니다 - 고객은 이메일 주소만으로 구매를 완료할 수 있습니다
- **즉시 액세스**: 결제 후 고객은 유료 제품과 마찬가지로 이메일을 통해 무료 제품에 즉시 액세스할 수 있습니다
- **혼합 카트**: 카트에 무료 제품과 유료 제품이 모두 포함되어 있는 경우 고객은 구매를 완료하기 위해 일반 결제 시스템(Stripe, PayPal 또는 SpectroCoin)을 사용해야 합니다

### 사용 사례

무료 제품은 다음에 적합합니다:
- 무료 다운로드 및 샘플
- 프로모션 콘텐츠 및 경품
- 디지털 제품의 체험판
- 무료 리소스 및 문서
- 리드 생성(이메일 주소 수집)

### 기술 세부 정보

- 무료 제품은 데이터베이스에서 `amount = 0`으로 식별됩니다
- **Dummy Payment** 공급자는 `amountTotal = 0`인 카트에 대해 자동으로 선택됩니다
- 모든 표준 기능이 무료 제품과 함께 작동합니다: 이메일 전달, 디지털 파일 다운로드, 라이선스 키 및 웹훅
- 무료 제품은 유료 제품과 마찬가지로 주문 기록 및 카트 관리에 포함됩니다
- 외부 결제 처리가 발생하지 않습니다 - Dummy Payment를 사용할 때 주문이 즉시 결제됨으로 표시됩니다

## 🧩&nbsp;&nbsp;개발자를 위해
백엔드는 Go 언어로 개발되었습니다. 프론트엔드 관리자 패널은 SvelteKit 및 TailwindCSS에서 작동합니다.  

개발을 단순화하는 여러 스크립트(./scripts 폴더에 있음)가 있습니다:  
`./scripts/golang` - go의 이전에 설치된 버전을 설치하거나 업데이트합니다(필요한 경우).  
`./scripts/migration` - 마이그레이션 작업을 돕습니다. 예를 들어 `./scripts/migration dev up` 명령은 ./migrations 폴더에서 새 마이그레이션을 적용한 다음 ./fixtures 폴더에서 마이그레이션을 구현합니다.  
`./scripts/sqlite` - 기존 데이터베이스를 최적화합니다.  
`./scripts/tools` - 개발에 필요한 환경을 설정합니다(필요한 경우).  
`./scripts/webscripts` - 프론트엔드(admin + site) 종속성(SvelteKit, Tailwind)을 최신 버전으로 업데이트합니다.  
`./scripts/clear` - 중단된 golang 또는 vite 프로세스를 제거합니다.  

> [!NOTE]
> `./scripts/migration dev up` 명령을 실행하는 것이 좋습니다. 데이터베이스에 테스트 데이터를 추가하여 작업을 더 쉽게 만듭니다. 예를 들어 제품을 만들고 테스트 이미지를 전송하며 관리자 패널에 액세스하기 위한 테스트 사용자를 만듭니다:  
> 로그인 - user@mail.com  
> 비밀번호 - Pass123

#### 관리자 패널(프론트엔드)
관리자 패널의 웹 인터페이스를 개발하려면 myCart 서버를 시작해야 합니다(예: 프로젝트 루트에서 `go run ./cmd/main.go serve` 명령 실행).
모든 코드는 ./web/admin 폴더에 있습니다. `cd ./web/admin && bun run dev` 명령은 관리자 패널 웹 인터페이스의 개발 서버를 시작합니다. 기본적으로 http://localhost:5173/_/에서 사용할 수 있습니다.

#### 기본 사이트(프론트엔드)
기본 사이트의 웹 인터페이스를 개발하려면 myCart 서버를 시작해야 합니다(예: 프로젝트 루트에서 `go run ./cmd/main.go serve` 명령 실행).  
모든 코드는 `./web/site` 폴더에 있습니다. `cd ./web/site && bun run dev` 명령은 Vite 개발 서버를 시작합니다. `cd ./web/site && bun run build`를 실행하여 Go 바이너리가 `//go:embed`를 통해 임베드하는 프로덕션 번들을 생성합니다.

#### 사용자 정의 및 배포
사이트 디자인을 사용자 정의하고 Nginx를 사용하여 별도의 서버에 배포하는 방법에 대한 자세한 내용은 [사용자 정의 및 배포 가이드](/customization)를 참조하세요.

## 🗺️&nbsp;&nbsp;할 일 목록
`mycart`에는 [로드맵](https://github.com/users/shurco/projects/2)이 있으며 특정 순서로 문제를 처리하려고 노력하며 이러한 PR은 종종 예상치 못한 곳에서 들어와 지루한 왕복 커뮤니케이션으로 모든 초기 계획을 왜곡합니다.

- [x] 파일 형태의 제품
- [x] 라이선스 키 형태의 제품
- [ ] API를 통해 다른 사이트로 반환되는 제품(예: 라이선스 키)
- [x] <a href="#stripe">결제 Stripe</a>
- [x] <a href="#paypal">결제 PayPal</a>
- [x] 결제 PortOne
- [ ] 결제 Square
- [ ] 결제 Adyen
- [ ] 결제 Checkout
- [ ] Webhook을 통한 결제
- [x] <a href="#spectrocoin">암호화폐를 사용한 결제 지원(SpectroCoin)</a>
- [x] <a href="#coinbase">Coinbase Commerce 암호화 결제</a>
- [x] WebHook 지원(<a href="https://github.com/msalbrain" target="_blank">@nicksnyder</a>님이 <a href="https://github.com/shurco/mycart/pull/61" target="_blank">#61</a>에서)
- [x] <a href="#dummy-payment">Dummy Payment</a> (<a href="https://github.com/majiayu000" target="_blank">@majiayu000</a>님이 <a href="https://github.com/shurco/mycart/pull/261" target="_blank">#261</a>에서)


## 👍&nbsp;&nbsp;기여하기

**감사하다**고 말하고 싶거나 `mycart`의 활발한 개발을 지원하고 싶다면:

1. 프로젝트에 [GitHub Star](https://github.com/shurco/mycart/stargazers)를 추가하세요.
2. [Twitter에서](https://twitter.com/intent/tweet?text=%F0%9F%9B%92%20myCart%20-%20shopping-cart%20in%201%20file%20on%20%23Go%20https%3A%2F%2Fgithub.com%2Fshurco%2Fmycart) 프로젝트에 대해 트윗하세요.
3. [Medium](https://medium.com/), [Dev.to](https://dev.to/) 또는 개인 블로그에 리뷰 또는 튜토리얼을 작성하세요.
4. [커피 한 잔](https://github.com/sponsors/shurco)을 기부하여 프로젝트를 지원하세요.

이 프로젝트에 기여하는 방법에 대한 자세한 내용은 [기여 가이드](https://github.com/shurco/mycart/blob/master/.github/CONTRIBUTING.md)에서 확인할 수 있습니다.
