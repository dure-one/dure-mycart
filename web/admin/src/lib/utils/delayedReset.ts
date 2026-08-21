import { onDestroy } from 'svelte'

/**
 * Creates a reset callback that runs after `delayMs`, cancelling any
 * previously scheduled run. The pending timer is cleared on component
 * destroy so state is never written after unmount.
 *
 * Canonical usage (drawer close animation):
 *   const resetDrawer = createDelayedReset(DRAWER_CLOSE_DELAY_MS)
 *   function closeDrawer() {
 *     drawerOpen = false
 *     resetDrawer(() => { drawerMode = null })
 *   }
 */
export function createDelayedReset(delayMs: number) {
  let timer: ReturnType<typeof setTimeout> | undefined

  onDestroy(() => {
    if (timer !== undefined) clearTimeout(timer)
  })

  return (reset: () => void) => {
    if (timer !== undefined) clearTimeout(timer)
    timer = setTimeout(() => {
      timer = undefined
      reset()
    }, delayMs)
  }
}
