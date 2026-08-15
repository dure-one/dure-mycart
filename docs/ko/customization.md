# 디자인 커스터마이제이션 및 배포

## 사이트 디자인 변경

### 프론트엔드 구조

사이트 프론트엔드는 `web/site/` 디렉토리에 있으며 다음을 사용합니다:
- **SvelteKit** - SPA 생성을 위한 프레임워크
- **TailwindCSS v4** - 스타일링용
- **TypeScript** - 타입 안전성을 위해

### 커스터마이제이션을 위한 주요 파일

#### 1. 전역 스타일

`web/site/src/app.css` 파일에는 전역 스타일과 TailwindCSS 설정이 포함되어 있습니다:

```css
@import 'tailwindcss';
@plugin "@tailwindcss/forms";

/* Additional styles for product description */
.prod_desc > * + * {
  margin-top: 0.75em;
}
```

여기서 다음을 할 수 있습니다:
- TailwindCSS를 통해 색상 구성표 변경
- 커스텀 CSS 클래스 추가
- 타이포그래피 설정

#### 2. 컴포넌트

주요 컴포넌트는 `web/site/src/lib/components/`에 있습니다:

- **Header.svelte** - 로고, 소셜 네트워크, 장바구니가 있는 사이트 헤더
- **Footer.svelte** - 페이지 링크가 있는 사이트 푸터
- **ProductCard.svelte** - 상품 카드
- **CookieConsent.svelte** - GDPR을 준수하는 쿠키 동의 배너
- **NotFoundPage.svelte** - 404 오류 페이지 컴포넌트
- **Overlay.svelte** - 로딩 및 오류를 위한 오버레이 컴포넌트

Header 수정 예제:

```svelte
<!-- web/site/src/lib/components/Header.svelte -->
<header class="bg-white">
  <!-- Change bg-white to desired color, e.g., bg-blue-50 -->
  <div class="mx-auto flex h-16 max-w-screen-xl items-center gap-8 px-4 sm:px-6 lg:px-8">
    <!-- Your custom content -->
  </div>
</header>
```

#### 3. 레이아웃

메인 레이아웃은 `web/site/src/lib/layouts/MainLayout.svelte`에 있습니다. 여기서 전체 페이지 구조를 변경할 수 있습니다.

#### 4. 페이지

페이지는 `web/site/src/routes/`에 있습니다:
- `+page.svelte` - 홈페이지
- `[slug]/+page.svelte` - 콘텐츠 페이지
- `products/[slug]/+page.svelte` - 상품 페이지
- `cart/+page.svelte` - 장바구니

### 디자인 수정 프로세스

1. **개발 모드**

   변경 사항을 실시간으로 확인하려면 개발 서버를 시작하세요:

   ```bash
   # Start mycart server (in one terminal)
   ./mycart serve

   # Start frontend dev server (in another terminal)
   cd web/site
   bun run dev
   ```

   사이트는 `http://localhost:5273`에서 사용할 수 있습니다

2. **변경 작업**

   - `web/site/src/lib/components/`의 컴포넌트 수정
   - `web/site/src/app.css`의 스타일 수정
   - `web/site/src/routes/`의 페이지 수정

3. **프로덕션용 빌드**

   변경 후 프론트엔드를 빌드합니다:

   ```bash
   cd web/site
   bun run build
   ```

   빌드된 파일은 `web/site/build/`에 위치하며, 다음 mycart 빌드 시 바이너리에 자동으로 임베드됩니다.

4. **mycart 재빌드**

   컴파일된 바이너리를 사용하는 경우 다시 빌드하세요:

   ```bash
   go build -o mycart ./cmd/main.go
   ```

### TailwindCSS를 통한 커스터마이제이션

TailwindCSS를 통해 색상, 글꼴 및 기타 매개변수를 설정할 수 있습니다. `web/site/tailwind.config.js` 파일이 없다면 생성하세요:

```javascript
/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eff6ff',
          500: '#3b82f6',
          700: '#1d4ed8',
        },
      },
    },
  },
  plugins: [],
}
```

그런 다음 컴포넌트에서 이 색상을 사용합니다:

```svelte
<div class="bg-primary-500 text-white">
  Custom color
</div>
```

## Nginx를 사용하여 별도 서버에 배포

### 1단계: 프론트엔드 빌드

```bash
cd web/site
bun install
bun run build
```

### 2단계: Nginx 설정

Nginx 설정 파일 `/etc/nginx/sites-available/mycart`를 생성합니다:

```nginx
server {
    listen 80;
    server_name yourdomain.com;
    
    # Maximum upload file size
    client_max_body_size 20M;
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;

    # Frontend static files
    root /path/to/mycart/web/site/build;
    index index.html;

    # SPA routing - all other requests to index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Static files
    location /assets {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

### 3단계: 설정 활성화

```bash
sudo ln -s /etc/nginx/sites-available/mycart /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### HTTPS 설정

프로덕션 환경에서는 HTTPS 사용을 권장합니다. Let's Encrypt를 사용하세요:

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d yourdomain.com
```

Certbot이 HTTPS를 사용하도록 Nginx 설정을 자동으로 업데이트합니다.

## 별도 서버에서의 전체 설정 예제

### 디렉토리 구조

```
/var/www/
└── mycart-frontend/    # Built frontend
    └── build/           # Static files
```

### 배포 명령

```bash
# 1. Create directory
sudo mkdir -p /var/www/mycart-frontend

# 2. Build frontend
cd web/site
bun install
bun run build
sudo cp -r build/* /var/www/mycart-frontend/

# 3. Set permissions
sudo chown -R www-data:www-data /var/www/mycart-frontend

# 4. Configure Nginx
sudo nano /etc/nginx/sites-available/mycart
# (paste configuration from section above)

# 5. Activate Nginx configuration
sudo ln -s /etc/nginx/sites-available/mycart /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

이후 지정한 도메인에서 사이트를 사용할 수 있습니다.
