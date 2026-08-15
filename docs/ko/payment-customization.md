# 결제 인터페이스 커스터마이제이션

이 문서는 myCart에서 고객을 위한 결제 인터페이스의 모양과 동작을 커스터마이징하는 방법을 설명합니다.

## 개요

이 문서는 myCart에서 고객을 위한 결제 인터페이스의 모양과 동작을 커스터마이징하는 방법을 설명합니다.

## 주요 파일

### 장바구니 및 결제 수단 선택 페이지
**경로**: `web/site/src/routes/cart/+page.svelte`

결제 진행 중 고객에게 표시되는 메인 파일입니다. 여기서 다음을 커스터마이징할 수 있습니다:
- 결제 제공자 카드의 모양
- 결제 수단 표시 순서
- 추가 정보 (수수료, 로고, 배지)
- 레이아웃 (세로 목록 또는 그리드)

### 결제 결과 페이지
- **결제 성공**: `web/site/src/routes/cart/payment/success/+page.svelte`
- **결제 취소**: `web/site/src/routes/cart/payment/cancel/+page.svelte`

### 유틸리티 및 번역
- **결제 유틸리티**: `web/site/src/lib/utils/payment.ts`
- **장바구니 스토어**: `web/site/src/lib/stores/cart.ts`
- **번역**: `web/site/src/lib/i18n/locales/en.json` (및 다른 언어)

## 빠른 시작: 간단한 변경

### 1. 제공자 카드 스타일 변경

`web/site/src/routes/cart/+page.svelte`에서 찾으세요 (약 280~290번째 줄 부근):

```svelte
<label
  for="stripe"
  class="block cursor-pointer border-4 border-black bg-white p-6 peer-checked:border-yellow-300 peer-checked:bg-yellow-300"
>
```

Tailwind 클래스를 원하는 스타일로 변경하세요:

```svelte
<!-- Example: softer design -->
<label
  for="stripe"
  class="block cursor-pointer rounded-lg border-2 border-gray-300 bg-white p-6 shadow-md hover:shadow-xl peer-checked:border-blue-500 peer-checked:bg-blue-50"
>
```

### 2. 제공자 로고 추가

`web/site/static/assets/img/payments/`에 SVG 로고를 추가하세요:
- `stripe.svg`
- `paypal.svg`
- `portone.svg`
- `spectrocoin.svg`
- `coinbase.svg`

그런 다음 컴포넌트에서:

```svelte
<label for="stripe" class="...">
  <div class="flex items-center gap-4">
    <img src="/assets/img/payments/stripe.svg" alt="Stripe" class="h-10" />
    <div>
      <p class="text-xl font-bold">{t('cart.stripe')}</p>
      <p class="text-sm">{t('cart.stripeDescription')}</p>
    </div>
  </div>
</label>
```

### 3. 레이아웃을 그리드로 변경

`class="space-y-4"`를 그리드로 교체하세요:

```svelte
<fieldset class="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
  <!-- provider cards -->
</fieldset>
```

### 4. 배지 및 추천 표시 추가

```svelte
<label for="stripe" class="relative ...">
  <!-- Card content -->
  
  <!-- Recommendation badge -->
  <span class="absolute -top-2 left-1/2 -translate-x-1/2 rounded bg-green-500 px-3 py-1 text-xs font-bold text-white uppercase">
    Recommended
  </span>
</label>
```

## 고급 설정

### 제공자 설정 생성

`web/site/src/lib/config/payment.ts`를 생성하세요:

```typescript
export const PAYMENT_PROVIDER_ORDER = ['stripe', 'paypal', 'spectrocoin', 'coinbase'] as const;

export const PAYMENT_PROVIDER_INFO = {
  stripe: {
    name: 'Stripe',
    description: 'Credit/Debit Cards',
    icon: '/assets/img/payments/stripe.svg',
    badge: 'Recommended',
    fee: '2.9% + $0.30'
  },
  paypal: {
    name: 'PayPal',
    description: 'PayPal balance or cards',
    icon: '/assets/img/payments/paypal.svg',
    badge: 'Fast',
    fee: '3.4% + fixed'
  },
  spectrocoin: {
    name: 'SpectroCoin',
    description: 'Cryptocurrencies',
    icon: '/assets/img/payments/spectrocoin.svg',
    fee: 'From 1%'
  },
  coinbase: {
    name: 'Coinbase Commerce',
    description: 'Cryptocurrencies via Coinbase',
    icon: '/assets/img/payments/coinbase.svg',
    fee: '1%'
  }
};
```

### 재사용 가능한 컴포넌트 생성

`web/site/src/lib/components/PaymentProviderCard.svelte`를 생성하세요:

```svelte
<script lang="ts">
  interface Props {
    id: string
    name: string
    description: string
    icon?: string
    badge?: string
    fee?: string
    selected: boolean
    onSelect: (id: string) => void
  }
  
  let { id, name, description, icon, badge, fee, selected, onSelect }: Props = $props()
</script>

<div class="relative">
  <input 
    type="radio" 
    {id} 
    checked={selected} 
    onchange={() => onSelect(id)}
    class="peer hidden" 
  />
  <label
    for={id}
    class="block cursor-pointer border-4 border-black bg-white p-6 transition-all peer-checked:border-yellow-300 peer-checked:bg-yellow-300"
  >
    <div class="flex items-center gap-4">
      {#if icon}
        <img src={icon} alt={name} class="h-12 w-12" />
      {/if}
      <div class="flex-1">
        <p class="mb-1 text-xl font-black uppercase">{name}</p>
        <p class="text-lg">{description}</p>
        {#if fee}
          <p class="mt-1 text-sm text-gray-600">Fee: {fee}</p>
        {/if}
      </div>
    </div>
  </label>
  
  {#if badge}
    <span class="absolute -top-2 right-4 rounded bg-green-500 px-3 py-1 text-xs font-bold text-white uppercase">
      {badge}
    </span>
  {/if}
</div>
```

사용 예:

```svelte
<script lang="ts">
  import PaymentProviderCard from '$lib/components/PaymentProviderCard.svelte'
  import { PAYMENT_PROVIDER_INFO } from '$lib/config/payment'
</script>

<fieldset class="space-y-4">
  {#each Object.entries(payments).filter(([_, active]) => active) as [key, _]}
    {@const info = PAYMENT_PROVIDER_INFO[key]}
    <PaymentProviderCard
      id={key}
      {...info}
      selected={provider === key}
      onSelect={(id) => provider = id}
    />
  {/each}
</fieldset>
```

## 텍스트 커스터마이제이션

모든 텍스트는 `web/site/src/lib/i18n/locales/`에 저장되어 있습니다:

**en.json**:
```json
{
  "cart": {
    "stripe": "Credit/Debit Card",
    "stripeDescription": "Visa, Mastercard, Amex",
    "paypal": "PayPal",
    "paypalDescription": "PayPal account or card",
    "spectrocoin": "Cryptocurrency",
    "spectrocoinDescription": "Bitcoin, Ethereum, and more",
    "coinbase": "Coinbase Commerce",
    "coinbaseDescription": "Pay with Bitcoin, Ethereum, and other crypto",
    "paymentSecure": "🔒 All payments are secure and encrypted",
    "recommended": "Recommended"
  }
}
```

## 디자인 예제

### 미니멀 디자인

```svelte
<label
  for="stripe"
  class="flex items-center justify-between rounded-lg border border-gray-200 bg-white p-4 hover:bg-gray-50 peer-checked:border-blue-500 peer-checked:bg-blue-50"
>
  <div class="flex items-center gap-3">
    <img src="/assets/img/payments/stripe.svg" alt="Stripe" class="h-8" />
    <span class="font-medium">{t('cart.stripe')}</span>
  </div>
  <div class="h-5 w-5 rounded-full border-2 border-gray-300 peer-checked:border-blue-500 peer-checked:bg-blue-500"></div>
</label>
```

### 그리드형 카드 디자인

```svelte
<fieldset class="grid grid-cols-3 gap-4">
  <label
    for="stripe"
    class="flex cursor-pointer flex-col items-center rounded-xl border-2 border-gray-200 bg-white p-6 hover:border-blue-500 peer-checked:border-blue-500 peer-checked:ring-4 peer-checked:ring-blue-100"
  >
    <img src="/assets/img/payments/stripe.svg" alt="Stripe" class="mb-3 h-12" />
    <span class="text-center font-semibold">{t('cart.stripe')}</span>
  </label>
</fieldset>
```

### 상세 디자인

```svelte
<label
  for="stripe"
  class="block cursor-pointer rounded-lg border-2 border-gray-200 bg-white p-6 shadow-sm hover:shadow-md peer-checked:border-blue-500 peer-checked:shadow-lg"
>
  <div class="flex items-start gap-4">
    <img src="/assets/img/payments/stripe.svg" alt="Stripe" class="h-12 w-12" />
    <div class="flex-1">
      <h3 class="mb-1 text-lg font-bold">{t('cart.stripe')}</h3>
      <p class="mb-2 text-sm text-gray-600">{t('cart.stripeDescription')}</p>
      <div class="flex items-center gap-2">
        <img src="/assets/img/cards/visa.svg" alt="Visa" class="h-6" />
        <img src="/assets/img/cards/mastercard.svg" alt="Mastercard" class="h-6" />
        <img src="/assets/img/cards/amex.svg" alt="Amex" class="h-6" />
      </div>
      <p class="mt-2 text-xs text-gray-500">Fee: 2.9% + $0.30</p>
    </div>
    <div class="text-green-500 peer-checked:block hidden">✓</div>
  </div>
</label>
```

## PortOne 설정

PortOne은 신용카드, 가상계좌, 모바일 결제를 지원하는 한국 결제 게이트웨이입니다.

### 설정 방법

1. **[PortOne 개발자 사이트](https://portone.io)에서 가입**
   - 계정 생성
   - 이메일 인증

2. **스토어 및 채널 생성**
   - 콘솔 → 스토어로 이동
   - 새 스토어 생성
   - 스토어 내에 결제 채널 생성
   - 활성화할 결제 수단 선택 (신용카드, 가상계좌 등)

3. **자격 증명 복사**
   - **Store ID**: `store-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` 형식
   - **Channel Key**: 채널의 고유 식별자
   - **API Secret**: V2 API 시크릿 키 (안전하게 보관하세요)

4. **웹훅 URL 설정**
   - PortOne 콘솔에서 Webhooks로 이동
   - 웹훅 URL 추가: `https://yourdomain.com/api/payment/portone/webhook`
   - 이벤트 선택: `Transaction.Paid`, `Transaction.VirtualAccountIssued`

5. **mycart 관리자 패널에 자격 증명 입력**
   - 설정 → 결제 → PortOne으로 이동
   - Store ID, Channel Key, API Secret 붙여넣기
   - Active 토글을 켜서 PortOne 결제 활성화
   - 저장 클릭

### 테스트 모드

개발 및 테스트 시:
- PortOne의 테스트 채널 자격 증명을 사용하세요
- 테스트 결제는 실제 금액이 청구되지 않습니다
- [PortOne 문서](https://developers.portone.io/docs/ko/readme)의 테스트 카드 번호를 사용하세요

### 지원되는 결제 수단

- **신용/체크카드**: Visa, Mastercard, JCB 등
- **가상계좌**: 가상계좌 번호를 통한 계좌 이체
- **모바일 결제**: 삼성페이, 애플페이, 네이버페이, 카카오페이
- **국내 계좌 이체**: 계좌 간 직접 이체

### 결제 흐름

다른 제공자와 달리 PortOne은 브라우저 SDK를 사용합니다:

1. 고객이 PortOne을 선택하고 결제를 클릭합니다
2. 프론트엔드가 스토어/채널 설정과 함께 `PortOne.requestPayment()`를 호출합니다
3. PortOne이 결제 UI를 표시합니다 (모달 또는 리디렉션)
4. 고객이 결제를 완료합니다
5. 프론트엔드가 결제 결과를 수신합니다
6. 프론트엔드가 백엔드로 결제를 검증합니다 (`POST /api/payment/portone/complete`)
7. 백엔드가 PortOne API로 검증하고 장바구니를 업데이트합니다

### 커스터마이제이션

`web/site/static/assets/img/payments/portone.svg`에 PortOne 로고를 추가하세요

커스텀 스타일링 예제:

```svelte
{#if payments.portone}
  <div>
    <input type="radio" bind:group={provider} value="portone" id="portone" class="peer hidden" />
    <label
      for="portone"
      class="block cursor-pointer border-4 border-black bg-white p-6 peer-checked:border-yellow-300 peer-checked:bg-yellow-300"
    >
      <div class="flex items-center gap-4">
        <img src="/assets/img/payments/portone.svg" alt="PortOne" class="h-10" />
        <div>
          <p class="mb-2 text-xl font-black tracking-tight text-black uppercase">{t('cart.portone')}</p>
          <p class="text-lg text-black">{t('cart.portoneDescription')}</p>
        </div>
      </div>
    </label>
  </div>
{/if}
```

## 권장 사항

1. **접근성**: 적절한 ARIA 속성을 사용하세요
2. **모바일 기기**: 모든 화면 크기에서 테스트하세요
3. **성능**: 아이콘을 최적화하세요 (SVG 사용)
4. **브랜딩**: 제공자의 로고를 사용할 때는 가이드라인을 따르세요
5. **UX**: 로딩 상태를 표시하고 오류를 처리하세요

## 스타일링

이 프로젝트는 Tailwind CSS를 사용합니다. 다음을 할 수 있습니다:
- 내장된 Tailwind 유틸리티 사용
- `web/site/src/app.css`에 커스텀 클래스 추가
- `web/site/tailwind.config.js`에서 설정

## 추가 도움말

또는 GitHub에 이슈를 열어주세요: https://github.com/shurco/mycart/issues
