import { apiGet, apiPost, apiUpdate, apiDelete } from './api'

export async function loadMessageThreads(filters?: { channel?: string; search?: string; page?: number; limit?: number }) {
	const params = new URLSearchParams(filters as any)
	return await apiGet(`/api/_/responder/messages?${params}`)
}

export async function loadCustomerMessages(customerId: string) {
	return await apiGet(`/api/_/responder/messages/${customerId}`)
}

export async function createMessage(data: {
	contact_address: string
	contact_type: string
	content: string
	direction: string
	channel: string
	delivery_status?: string
}) {
	return await apiPost('/api/_/responder/messages', data)
}

export async function linkContact(contactId: string, newCustomerId: string) {
	return await apiPost('/api/_/responder/messages/link-contact', {
		contact_id: contactId,
		new_customer_id: newCustomerId
	})
}

export async function markMessageRead(messageId: string) {
	return await apiUpdate(`/api/_/responder/messages/${messageId}/read`, {})
}

export async function loadWorkflows(filters?: { enabled?: boolean; search?: string; page?: number; limit?: number }) {
	const params = new URLSearchParams(filters as any)
	return await apiGet(`/api/_/responder/workflows?${params}`)
}

export async function getWorkflow(id: string) {
	return await apiGet(`/api/_/responder/workflows/${id}`)
}

export async function saveWorkflow(workflow: {
	id?: string
	name: string
	description?: string
	content: string
	enabled: boolean
	tags?: string
	version?: string
	author?: string
}) {
	if (workflow.id) {
		return await apiUpdate(`/api/_/responder/workflows/${workflow.id}`, workflow)
	} else {
		return await apiPost('/api/_/responder/workflows', workflow)
	}
}

export async function deleteWorkflow(id: string) {
	return await apiDelete(`/api/_/responder/workflows/${id}`)
}

export async function loadCrontabJobs() {
	return await apiGet('/api/_/settings/crontab')
}

export async function updateCrontabJob(jobId: string, updates: { enabled?: boolean; interval?: string }) {
	return await apiUpdate(`/api/_/settings/crontab/${jobId}`, updates)
}

export async function loadResponderSettings() {
	return await apiGet('/api/_/settings/responder')
}

export async function saveResponderSettings(settings: {
	xmpp_jid: string
	xmpp_password: string
	xmpp_server?: string
	xmpp_port?: number
	xmpp_websocket_url?: string
	xmpp_bosh_url?: string
}) {
	return await apiUpdate('/api/_/settings/responder', settings)
}

export async function testXMPPConnection(settings: {
	xmpp_jid: string
	xmpp_password: string
	xmpp_server?: string
	xmpp_port?: number
	xmpp_websocket_url?: string
	xmpp_bosh_url?: string
}) {
	return await apiPost('/api/_/settings/responder/test-connection', settings)
}

export async function checkCrontabStatus() {
	return await apiGet('/api/_/settings/crontab/status')
}

export async function installCrontab() {
	return await apiPost('/api/_/settings/crontab/install', {})
}

export async function uninstallCrontab() {
	return await apiPost('/api/_/settings/crontab/uninstall', {})
}
