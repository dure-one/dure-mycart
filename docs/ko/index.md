---
layout: home

hero:
  name: myCart
  text: 하나의 파일로 된 쇼핑카트
  tagline: Go + SQLite + SvelteKit으로 구축된 단일 바이너리 전자상거래 솔루션
  actions:
    - theme: brand
      text: 시작하기
      link: /ko/readme
    - theme: alt
      text: GitHub에서 보기
      link: https://github.com/shurco/mycart
    - theme: alt
      text: API 문서
      link: /swagger/

features:
  - icon: 📦
    title: 단일 바이너리
    details: 관리자 패널, 스토어프론트, API가 하나의 실행 파일에 모두 포함
  - icon: 🗄️
    title: SQLite 데이터베이스
    details: 외부 종속성이나 설정이 필요 없는 임베디드 데이터베이스
  - icon: ⚡
    title: Go 백엔드
    details: Go 1.26으로 구축된 빠르고 안정적이며 배포하기 쉬운 백엔드
  - icon: 🎨
    title: SvelteKit 프론트엔드
    details: SvelteKit과 Tailwind CSS로 구축된 현대적인 관리자 패널 및 스토어프론트
  - icon: 🔌
    title: 완전한 API
    details: 완전한 Swagger/OpenAPI 문서가 포함된 RESTful API
  - icon: ✅
    title: E2E 테스트
    details: 안정성을 위한 포괄적인 Playwright 테스트 커버리지
---

## 빠른 시작

```bash
# 다운로드 및 실행
./mycart serve

# 관리자 패널 접속
open http://localhost:8080/_/

# 스토어프론트 접속
open http://localhost:8080/
```

기본 인증 정보: `user@mail.com` / `Pass123`

## 아키텍처

myCart는 다음으로 구성된 모놀리식 전자상거래 백엔드입니다:

- **Go 1.26** - 백엔드 런타임
- **Fiber v3** - HTTP 프레임워크
- **SQLite** - modernc.org/sqlite를 통한 임베디드 데이터베이스 (순수 Go, CGO 불필요)
- **SvelteKit** - 관리자 및 스토어프론트 SPA
- **Goose** - 데이터베이스 마이그레이션

모든 구성 요소는 `go:embed`를 사용하여 단일 바이너리에 임베디드됩니다.

## 문서

- **[시작하기](/ko/readme)** - 설치 및 설정 가이드
- **[API 문서](/swagger/)** - 완전한 Swagger/OpenAPI 문서
- **[E2E 테스트 리포트](/e2e/)** - Playwright 테스트 결과
- **[커스터마이제이션](/ko/customization)** - 스토어 커스터마이즈
- **[GitHub 저장소](https://github.com/shurco/mycart)** - 소스 코드 및 이슈
