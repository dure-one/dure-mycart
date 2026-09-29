import { loadWorkflows } from '$lib/utils/responder'
import type { PageLoad } from './$types'

export const load: PageLoad = async () => {
	const response = await loadWorkflows()

	return {
		workflows: response.success ? response.result?.workflows || [] : [],
		total: response.success ? response.result?.total || 0 : 0
	}
}
