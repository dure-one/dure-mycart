<script lang="ts">
  import { onMount } from 'svelte'

  interface Props {
    open?: boolean
  }

  let { open = $bindable(false) }: Props = $props()

  interface SellerInfo {
    business_name: string
    representative: string
    customer_service: string
    business_reg_number: string
    business_address: string
    ecommerce_license: string
    email: string
  }

  let captchaVerified = $state(false)
  let sellerInfo = $state<SellerInfo | null>(null)
  let loading = $state(false)
  let error = $state('')
  let captchaToken = $state('')
  let masterImage = $state('')
  let thumbImage = $state('')
  let currentAngle = $state(0)
  let accessToken = $state('')

  async function loadCaptcha() {
    loading = true
    error = ''
    try {
      const response = await fetch('/api/sellerinfo/captcha')
      if (!response.ok) throw new Error('Failed to load captcha')
      const data = await response.json()
      masterImage = data.master_image
      thumbImage = data.thumb_image
      captchaToken = data.token
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load captcha'
    } finally {
      loading = false
    }
  }

  async function verifyCaptcha() {
    loading = true
    error = ''
    try {
      const response = await fetch('/api/sellerinfo/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          token: captchaToken,
          angle: currentAngle
        })
      })
      if (!response.ok) {
        const data = await response.json()
        throw new Error(data.message || 'Verification failed')
      }
      const data = await response.json()
      accessToken = data.access_token
      captchaVerified = true
      await loadSellerInfo()
    } catch (err) {
      error = err instanceof Error ? err.message : 'Verification failed'
      await loadCaptcha() // Reload captcha on failure
    } finally {
      loading = false
    }
  }

  async function loadSellerInfo() {
    loading = true
    error = ''
    try {
      const response = await fetch('/api/sellerinfo', {
        headers: { 'X-Access-Token': accessToken }
      })
      if (!response.ok) throw new Error('Failed to load seller info')
      const data = await response.json()
      sellerInfo = data.seller_info
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load seller info'
    } finally {
      loading = false
    }
  }

  function closeModal() {
    open = false
    captchaVerified = false
    sellerInfo = null
    error = ''
    captchaToken = ''
    masterImage = ''
    thumbImage = ''
    currentAngle = 0
    accessToken = ''
  }

  onMount(() => {
    if (open && !masterImage) {
      loadCaptcha()
    }
  })

  $effect(() => {
    if (open && !masterImage && !loading) {
      loadCaptcha()
    }
  })
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50" onclick={closeModal}>
    <div
      class="relative max-h-[90vh] w-full max-w-2xl overflow-y-auto border-4 border-yellow-300 bg-black p-8 shadow-[8px_8px_0px_0px_rgba(255,235,59,1)]"
      onclick={(e) => e.stopPropagation()}
    >
      <button
        onclick={closeModal}
        class="absolute top-4 right-4 border-2 border-white bg-white p-2 text-black transition-all duration-200 hover:-translate-x-1 hover:-translate-y-1 hover:shadow-[4px_4px_0px_0px_rgba(255,235,59,1)]"
        aria-label="Close"
      >
        <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>

      <h2 class="mb-6 border-b-4 border-yellow-300 pb-4 text-2xl font-black uppercase text-yellow-300">
        판매자 정보
      </h2>

      {#if error}
        <div class="mb-4 border-2 border-red-500 bg-red-500 bg-opacity-20 p-4 text-red-300">
          {error}
        </div>
      {/if}

      {#if loading}
        <div class="py-8 text-center text-yellow-300">
          <div class="mb-2 text-xl font-black">Loading...</div>
        </div>
      {:else if !captchaVerified}
        <div class="space-y-6">
          <div class="text-center">
            <p class="mb-4 text-sm font-black uppercase text-white">회전하여 이미지를 맞춰주세요</p>
            {#if masterImage && thumbImage}
              <div class="relative mx-auto mb-4 inline-block">
                <img src={masterImage} alt="Captcha" class="border-4 border-white" />
                <div
                  class="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2"
                  style="transform: translate(-50%, -50%) rotate({currentAngle}deg)"
                >
                  <img src={thumbImage} alt="Thumb" class="border-2 border-yellow-300" />
                </div>
              </div>
              <div class="mb-4">
                <label class="mb-2 block text-xs font-black uppercase text-yellow-300">
                  각도: {currentAngle}°
                </label>
                <input
                  type="range"
                  min="0"
                  max="360"
                  bind:value={currentAngle}
                  class="w-full"
                />
              </div>
              <button
                onclick={verifyCaptcha}
                class="w-full border-4 border-yellow-300 bg-yellow-300 px-6 py-3 text-lg font-black uppercase text-black transition-all duration-200 hover:-translate-x-1 hover:-translate-y-1 hover:shadow-[6px_6px_0px_0px_rgba(255,255,255,1)]"
              >
                확인
              </button>
            {/if}
          </div>
        </div>
      {:else if sellerInfo}
        <div class="space-y-2 text-white">
          <div class="border-b-2 border-gray-700 pb-2">
            <span class="font-black text-yellow-300">상호명:</span>
            <span class="ml-2">{sellerInfo.business_name}</span>
          </div>
          <div class="border-b-2 border-gray-700 pb-2">
            <span class="font-black text-yellow-300">대표자:</span>
            <span class="ml-2">{sellerInfo.representative}</span>
          </div>
          <div class="border-b-2 border-gray-700 pb-2">
            <span class="font-black text-yellow-300">고객센터:</span>
            <span class="ml-2">{sellerInfo.customer_service}</span>
          </div>
          <div class="border-b-2 border-gray-700 pb-2">
            <span class="font-black text-yellow-300">사업자등록번호:</span>
            <span class="ml-2">{sellerInfo.business_reg_number}</span>
          </div>
          <div class="border-b-2 border-gray-700 pb-2">
            <span class="font-black text-yellow-300">사업장 소재지:</span>
            <span class="ml-2">{sellerInfo.business_address}</span>
          </div>
          <div class="border-b-2 border-gray-700 pb-2">
            <span class="font-black text-yellow-300">통신판매업번호:</span>
            <span class="ml-2">{sellerInfo.ecommerce_license}</span>
          </div>
          <div class="pb-2">
            <span class="font-black text-yellow-300">e-mail:</span>
            <span class="ml-2">{sellerInfo.email}</span>
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}
