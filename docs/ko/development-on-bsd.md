# BSD 시스템에서의 개발 (OpenBSD/FreeBSD)

이 가이드는 Tailwind CSS v4와 Patchright E2E 테스트를 위한 BSD 관련 설정 요구사항을 다룹니다.

## 개요

Tailwind CSS v4는 성능 향상을 위해 PostCSS에서 Rust 기반 네이티브 바이너리로 마이그레이션되었습니다. 두 개의 네이티브 모듈이 필요합니다:

- **lightningcss** - CSS 파싱, 변환, 압축
- **@tailwindcss/oxide** - Tailwind CSS v4 코어 엔진

이 패키지들은 OpenBSD/FreeBSD용 사전 빌드된 바이너리를 제공하지 않으므로 소스에서 직접 빌드해야 합니다.

## 사전 요구사항

```bash
# Install Rust toolchain
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Verify installation
cargo --version
rustc --version
```

## lightningcss 빌드

### 1. 클론 및 빌드

```bash
# Clone repository
cd /tmp
git clone https://github.com/parcel-bundler/lightningcss.git
cd lightningcss

# Build the Node.js addon
cargo build --release -p lightningcss_node

# Build the CLI tool (optional)
cargo build --release --bin lightningcss --features cli
```

### 2. 바이너리 설치

```bash
# Create storage directory
mkdir -p ~/.local/lib/node-native-openbsd

# Copy Node.js addon
cp target/release/liblightningcss_node.so \
   ~/.local/lib/node-native-openbsd/lightningcss.openbsd-x64.node

# Copy CLI tool (optional)
cp target/release/lightningcss ~/.cargo/bin/
```

**참고:** `.so` 공유 라이브러리는 Node.js NAPI-RS 로딩을 위해 플랫폼 접미사가 붙은 `.node`로 이름이 변경됩니다.

## @tailwindcss/oxide 빌드

### 1. 클론 및 빌드

```bash
# Clone repository
cd /tmp
git clone https://github.com/tailwindlabs/tailwindcss.git
cd tailwindcss

# Build the oxide crate
cargo build --release -p tailwind-oxide
```

### 2. 바이너리 설치

```bash
# Copy to storage directory
cp target/release/libtailwind_oxide.so \
   ~/.local/lib/node-native-openbsd/tailwindcss-oxide.openbsd-x64.node
```

## 의존성 설치

### --legacy-peer-deps로 npm install

Vite 7과 (Vite 6을 요구하는) `@sveltejs/vite-plugin-svelte@5.1.1` 사이의 피어 의존성 충돌로 인해 `--legacy-peer-deps` 플래그를 사용해야 합니다:

```bash
# In web/admin
cd web/admin
npm install --legacy-peer-deps

# In web/site
cd web/site
npm install --legacy-peer-deps
```

**필요한 이유:**
- Vite 7은 최신 기능을 위해 필요합니다
- `@sveltejs/vite-plugin-svelte@5.1.1`은 Vite 6에 대한 피어 의존성을 선언합니다
- 버전 불일치에도 불구하고 두 패키지는 함께 정상적으로 작동합니다
- `--legacy-peer-deps`는 npm의 엄격한 피어 의존성 검사를 우회합니다

### SvelteKit Sync

의존성 설치 후, TypeScript 설정을 생성하기 위해 SvelteKit sync를 실행하세요:

```bash
# In web/admin
cd web/admin
./node_modules/.bin/svelte-kit sync

# In web/site
cd web/site
./node_modules/.bin/svelte-kit sync
```

## 자동화된 설치

바이너리를 한 번 빌드한 후에는 postinstall 스크립트를 사용하여 `npm install` 이후 자동으로 복원할 수 있습니다:

### 수동 실행

```bash
# From project root
node scripts/postinstall-openbsd-natives.js
```

### package.json에 추가 (선택 사항)

```json
{
  "scripts": {
    "postinstall": "node scripts/postinstall-openbsd-natives.js"
  }
}
```

## Postinstall 스크립트가 하는 일

이 스크립트(`scripts/postinstall-openbsd-natives.js`)는 두 가지 작업을 수행합니다:

### 1. 네이티브 바이너리 복사

`~/.local/lib/node-native-openbsd/`에서 사전 빌드된 바이너리를 다음 위치로 복사합니다:

- `web/admin/node_modules/@tailwindcss/node/node_modules/lightningcss/lightningcss.openbsd-x64.node`
- `web/admin/node_modules/@tailwindcss/oxide/tailwindcss-oxide.openbsd-x64.node`
- `web/site/node_modules/@tailwindcss/node/node_modules/lightningcss/lightningcss.openbsd-x64.node`
- `web/site/node_modules/@tailwindcss/oxide/tailwindcss-oxide.openbsd-x64.node`

### 2. JavaScript 로더 패치

**@tailwindcss/oxide/index.js의 경우:**

"Unsupported OS" 오류 앞에 OpenBSD/FreeBSD 플랫폼 감지 코드를 추가합니다:

```javascript
} else if (process.platform === 'openbsd' || process.platform === 'freebsd') {
  if (process.arch === 'x64') {
    try {
      return require('./tailwindcss-oxide.openbsd-x64.node')
    } catch (e) {
      loadErrors.push(e)
    }
  } else {
    loadErrors.push(new Error(`Unsupported architecture on BSD: ${process.arch}`))
  }
} else {
  loadErrors.push(new Error(`Unsupported OS: ${process.platform}, architecture: ${process.arch}`))
}
```

**lightningcss의 경우:**

패치가 필요하지 않습니다 — lightningcss는 NAPI-RS 네이밍 규칙을 통해 커스텀 플랫폼 바이너리를 이미 지원합니다.

## 패치가 필요한 이유

NAPI-RS(Rust-to-Node.js 바인딩)는 플랫폼 감지 코드를 자동으로 생성하지만, "공식적으로 지원되는" 플랫폼(Windows, macOS, Linux)에 대해서만 생성합니다. 생성된 `index.js`에는 `process.platform`을 확인하는 if/else 체인이 포함되어 있습니다:

```javascript
if (process.platform === 'darwin') {
  // macOS loading
} else if (process.platform === 'linux') {
  // Linux loading
} else if (process.platform === 'win32') {
  // Windows loading
} else {
  // Error: Unsupported OS
}
```

**패치는 최종 오류 이전에 OpenBSD/FreeBSD 분기를 추가**하여 Node.js가 `.openbsd-x64.node` 바이너리를 로드하도록 합니다.

## 파일 네이밍 규칙

네이티브 Node.js addon은 다음 패턴을 따릅니다:

```
<package-name>.<platform>-<arch>.node
```

예시:
- `lightningcss.openbsd-x64.node`
- `tailwindcss-oxide.openbsd-x64.node`
- `tailwindcss-oxide.linux-x64.node` (비교용)

## 검증

postinstall 스크립트 실행 후 빌드가 정상 작동하는지 확인하세요:

```bash
# Test admin build
cd web/admin
npm run build

# Test site build
cd web/site
npm run build
```

예상 출력:
```
vite v7.3.6 building ssr environment for production...
✓ 305 modules transformed.
✓ built in X.XXs
```

## 문제 해결

### npm ERESOLVE unable to resolve dependency tree

**오류:**
```
npm error ERESOLVE unable to resolve dependency tree
npm error peer vite@"^6.0.0" from @sveltejs/vite-plugin-svelte@5.1.1
```

**원인:** Vite 7과 `@sveltejs/vite-plugin-svelte` 간의 피어 의존성 충돌.

**해결:**
```bash
npm install --legacy-peer-deps
```

### "Cannot find module '../lightningcss.openbsd-x64.node'"

**원인:** 바이너리가 node_modules에 복사되지 않았습니다.

**해결:**
```bash
node scripts/postinstall-openbsd-natives.js
```

### "Cannot find native binding"

**원인:** oxide의 index.js가 패치되지 않았습니다.

**해결:**
```bash
# Re-run postinstall (includes patching)
node scripts/postinstall-openbsd-natives.js
```

### "Unsupported OS: openbsd"로 빌드 실패

**원인:** 패치가 적용되지 않았거나 npm install에 의해 덮어써졌습니다.

**해결:**
```bash
# Always run postinstall after npm install
npm install --legacy-peer-deps
node scripts/postinstall-openbsd-natives.js
```

### Cargo 빌드 실패

**일반적인 원인:**

1. **Rust 미설치:** https://rustup.rs 에서 설치하세요
2. **lightningcss 저장소에서 npm install 중 Puppeteer 오류:**
   - 클론한 저장소에서 `npm build`를 사용하지 마세요
   - 대신 `cargo build`를 직접 사용하세요
3. **오래된 Rust 버전:** `rustup update`로 업데이트하세요

## 대안: 수동 설치

postinstall 스크립트가 작동하지 않으면 수동으로 복사하고 패치하세요:

```bash
# Copy binaries
cp ~/.local/lib/node-native-openbsd/lightningcss.openbsd-x64.node \
   web/admin/node_modules/@tailwindcss/node/node_modules/lightningcss/

cp ~/.local/lib/node-native-openbsd/tailwindcss-oxide.openbsd-x64.node \
   web/admin/node_modules/@tailwindcss/oxide/

# Patch oxide (lightningcss needs no patch)
# Edit web/admin/node_modules/@tailwindcss/oxide/index.js
# Add the OpenBSD/FreeBSD branch shown above
```

## 유지 보수

`@tailwindcss/oxide` 또는 `lightningcss`를 업데이트하는 `npm install` 이후:

```bash
# Re-apply patches and binaries
npm install --legacy-peer-deps
node scripts/postinstall-openbsd-natives.js
```

**참고:** Vite 7과 SvelteKit 플러그인 간의 피어 의존성 충돌을 피하려면 `npm install` 실행 시 항상 `--legacy-peer-deps`를 사용하세요.

자동으로 실행되도록 package.json 스크립트에 추가하는 것을 고려하세요.

## Patchright를 사용한 E2E 테스트

### 개요

Patchright(Playwright의 포크)는 BSD 시스템에서 비디오 녹화를 위해 플랫폼별 패치와 ffmpeg 설정이 필요합니다.

### Patchright 플랫폼 패치

`scripts/patch-patchright-openbsd.js` 스크립트는 `npm install` 후 자동으로 Patchright를 패치하여 OpenBSD/FreeBSD를 지원합니다:

**적용되는 패치:**
1. 캐시 디렉토리 감지 (3곳)
2. 사용자 에이전트 플랫폼 매핑
3. 창 인셋(window insets) 설정
4. Chrome 채널 실행 파일 경로
5. BSD 시스템용 플랫폼 감지
6. ffmpeg 플랫폼 매핑

이 패치들은 `package.json`의 `postinstall` 훅을 통해 자동으로 실행됩니다.

### 비디오 녹화를 위한 ffmpeg 설정

#### 문제

Playwright/Patchright는 번들된 ffmpeg 바이너리가 다음 위치에 있을 것으로 예상합니다:
```
~/.cache/ms-playwright/ffmpeg-1011/ffmpeg-linux
```

하지만 Patchright는 **Linux용 ffmpeg 바이너리를 배포하지 않습니다** (`linux-x64`로 매핑되는 OpenBSD 포함). `patchright install ffmpeg` 명령은 Linux 시스템에 ffmpeg가 시스템 전역으로 설치되어 있을 것이라고 가정하기 때문에 실패합니다.

#### 해결책

시스템의 ffmpeg는 필요한 모든 코덱을 포함하여 완전히 작동합니다. Patchright가 예상하는 위치로 심볼릭 링크만 걸어주면 됩니다:

```bash
# Install ffmpeg (if not already installed)
doas pkg_add ffmpeg

# Create Patchright cache directory
mkdir -p ~/.cache/ms-playwright/ffmpeg-1011

# Symlink system ffmpeg to expected location
ln -sf /usr/local/bin/ffmpeg ~/.cache/ms-playwright/ffmpeg-1011/ffmpeg-linux
```

#### 검증

```bash
# Verify symlink
ls -la ~/.cache/ms-playwright/ffmpeg-1011/

# Should show:
# lrwxr-xr-x  1 user  group  21 Jul 24 08:55 ffmpeg-linux -> /usr/local/bin/ffmpeg
```

#### 비디오 녹화 설정

`playwright.config.ts`의 비디오 녹화 옵션:

```typescript
video: 'off',                 // Disabled - no video recording
video: 'on',                  // Always record (requires ffmpeg)
video: 'retain-on-failure',   // Only save videos when tests fail (recommended)
video: 'on-first-retry',      // Only record on retries
```

**권장:** `retain-on-failure` - 디스크 공간을 절약하면서 실패한 테스트의 디버깅용 비디오는 보존합니다.

### E2E 테스트를 위한 시스템 요구사항

```bash
# Required packages
doas pkg_add chromium ffmpeg

# Verify installations
chrome --version    # Should show Chromium version
ffmpeg -version     # Should show ffmpeg 8.x with codecs
```

### E2E 테스트 실행

```bash
# Build and run all tests
npm run test:e2e

# Run without rebuilding
npm run test:e2e:nobuild

# Run with UI (interactive mode)
npm run test:e2e:ui

# Run with debugging
npm run test:e2e:debug

# View test report
npm run test:e2e:report
```

### E2E 테스트 문제 해결

#### "Executable doesn't exist at .../ffmpeg-linux"

**원인:** ffmpeg 심볼릭 링크가 생성되지 않았습니다.

**해결:**
```bash
mkdir -p ~/.cache/ms-playwright/ffmpeg-1011
ln -sf /usr/local/bin/ffmpeg ~/.cache/ms-playwright/ffmpeg-1011/ffmpeg-linux
```

#### "ERROR: Patchright does not support ffmpeg on linux-x64"

**원인:** 예상된 동작입니다 — Patchright는 Linux용 ffmpeg를 배포하지 않습니다.

**해결:** `patchright install ffmpeg` 대신 위의 심볼릭 링크 방식을 사용하세요.

#### 테스트에서 Chrome을 시작하지 못함

**원인:** Chromium이 설치되지 않았거나 경로가 잘못되었습니다.

**해결:**
```bash
doas pkg_add chromium
which chrome  # Should show /usr/local/bin/chrome
```

#### Patchright 패치가 적용되지 않음

**원인:** npm install은 실행되었지만 postinstall 훅이 실행되지 않았습니다.

**해결:**
```bash
npm run postinstall
# Should show: "✅ Successfully applied X patches for openbsd"
```

### 시스템 ffmpeg가 작동하는 이유

OpenBSD의 ffmpeg(버전 8.x)는 Playwright에 필요한 모든 코덱을 포함합니다:

- **비디오:** H.264(libx264), H.265(libx265), VP8/VP9(libvpx), AV1(libaom, libdav1d, libsvtav1)
- **오디오:** AAC, Opus, Vorbis, MP3(libmp3lame)
- **컨테이너:** MP4, WebM, Matroska

이는 OpenBSD ports 팀에 의해 유지 관리되고 최신 상태로 유지되므로 번들 바이너리보다 더 신뢰할 수 있습니다.

## 관련 파일

- `scripts/postinstall-openbsd-natives.js` - Tailwind용 자동화된 패치 및 바이너리 설치 스크립트
- `scripts/patch-patchright-openbsd.js` - E2E 테스트를 위한 Patchright 플랫폼 패치
- `playwright.config.ts` - E2E 테스트 설정
- `~/.local/lib/node-native-openbsd/` - 사전 빌드된 Tailwind 바이너리 저장소
- `~/.cache/ms-playwright/` - Patchright 캐시 디렉토리

## 기술 배경

### Tailwind v4가 네이티브 바이너리를 필요로 하는 이유

Tailwind CSS v4 아키텍처:

- **v3:** PostCSS 플러그인 (순수 JavaScript, 느림)
- **v4:** Rust 기반 엔진 (컴파일된 네이티브 코드, 약 100배 빠름)

Rust 코드는 플랫폼별 공유 라이브러리(BSD/Linux에서는 `.so`, macOS에서는 `.dylib`, Windows에서는 `.dll`)로 컴파일되고 NAPI-RS를 사용하여 Node.js 네이티브 addon(`.node` 파일)으로 래핑됩니다.

### .so → .node 이름 변경 이유

Node.js는 `.node` 확장자를 가진 네이티브 addon을 예상합니다. Rust 빌드는 다음을 생성합니다:

- `liblightningcss_node.so` (Rust/Cargo 네이밍)
- `libtailwind_oxide.so` (Rust/Cargo 네이밍)

NAPI-RS 로더가 예상하는 형식:

- `<package>.<platform>-<arch>.node` (Node.js 네이밍)

복사 단계에서 Node.js 규칙에 맞게 이 파일들의 이름을 변경하고 이동시킵니다.

### NAPI-RS 로딩 메커니즘

1. 패키지의 `index.js`가 `process.platform`과 `process.arch`를 확인합니다
2. 파일명을 구성합니다: `<package>.${platform}-${arch}.node`
3. `require('./constructed-filename')`을 시도합니다
4. 찾지 못하면 대체 플랫폼을 시도합니다
5. 모두 실패하면 "Cannot find native binding" 오류를 발생시킵니다

패치는 2.5단계를 추가합니다: "BSD라면 `.openbsd-x64.node`를 로드"
