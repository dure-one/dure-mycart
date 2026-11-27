import { redirect } from '@sveltejs/kit'
import { resolve } from '$app/paths'
import type { PageLoad } from './$types'

export const load: PageLoad = () => {
  throw redirect(302, resolve('/settings/main'))
}
