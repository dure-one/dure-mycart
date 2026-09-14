import { apiGet, apiPost, apiUpdate, apiDelete, showMessage } from './index'
import type { ApiResponse } from '$lib/types/api'

/**
 * runCall performs a call and reports both halves of what happened: the message
 * for the operator, and whether the call succeeded.
 *
 * The outcome has to be reported separately because it cannot be read off the
 * payload. A delete succeeds and returns nothing, and a call that failed returns
 * nothing either — so `result` alone cannot tell the two apart, and a caller
 * that only wants to know whether to reload the list behind a delete has to ask
 * for the flag rather than for the result.
 */
async function runCall<T>(
  apiCall: () => Promise<ApiResponse<T>>,
  successMessage?: string,
  errorMessage?: string
): Promise<{ ok: boolean; result: T | null }> {
  try {
    const res = await apiCall()
    if (res.success) {
      if (successMessage) {
        showMessage(successMessage, 'connextSuccess')
      }
      return { ok: true, result: res.result || null }
    }
    showMessage(res.message || errorMessage || 'Operation failed', 'connextError')
    return { ok: false, result: null }
  } catch (error) {
    showMessage(errorMessage || 'Network error', 'connextError')
    return { ok: false, result: null }
  }
}

export async function handleApiCall<T>(
  apiCall: () => Promise<ApiResponse<T>>,
  successMessage?: string,
  errorMessage?: string
): Promise<T | null> {
  return (await runCall(apiCall, successMessage, errorMessage)).result
}

export async function loadData<T>(url: string, errorMessage = 'Failed to load data'): Promise<T | null> {
  return handleApiCall(() => apiGet<T>(url), undefined, errorMessage)
}

export async function saveData<T, D = Partial<T>>(
  url: string,
  data: D,
  isUpdate: boolean,
  successMessage = 'Data saved',
  errorMessage = 'Failed to save data'
): Promise<T | null> {
  const apiCall = isUpdate ? () => apiUpdate<T>(url, data) : () => apiPost<T>(url, data)
  return handleApiCall(apiCall, successMessage, errorMessage)
}

export async function deleteData(
  url: string,
  successMessage = 'Deleted successfully',
  errorMessage = 'Failed to delete'
): Promise<boolean> {
  return (await runCall(() => apiDelete(url), successMessage, errorMessage)).ok
}

export async function toggleActive<T = any>(
  url: string,
  successMessage = 'Status updated',
  errorMessage = 'Failed to update status'
): Promise<T | null> {
  return (await runCall(() => apiUpdate<T>(url, {}), successMessage, errorMessage)).result
}
