export interface ApiResponse<T = any> {
  success: boolean
  message?: string
  result?: T
  status?: number
}

export interface CartValidationError {
  item_index: number
  product_id: string
  variant_id?: string
  error_type: string
  requested_qty: number
  available_qty: number
  requested_unit_price: number
  current_unit_price: number
  requested_total: number
  current_total: number
}

export interface CorrectedCartItem {
  product_id: string
  variant_id?: string
  quantity: number
  unit_price: number
  available: boolean
}

export interface CartCreateResponse {
  cart_id: string
  amount_total: number
  currency: string
}

export interface CartValidationResponse {
  validation_errors: CartValidationError[]
  corrected_cart: CorrectedCartItem[]
}

export interface RequestOptions extends RequestInit {
  credentials?: RequestCredentials
  method: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: string | FormData
  headers?: HeadersInit
}
