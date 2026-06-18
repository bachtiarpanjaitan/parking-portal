// API client — all requests go through this axios instance which
// automatically attaches the JWT from localStorage.

import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'

const BASE = import.meta.env.VITE_API_URL || ''
const STORAGE_KEY = 'auth-v1'

export const api = axios.create({
    baseURL: BASE,
    headers: { 'Content-Type': 'application/json' },
    // Give the proxy/gateway a chance to fail fast in dev.
    timeout: 10_000,
})

function readToken(): string | null {
    try {
        const raw = localStorage.getItem(STORAGE_KEY)
        if (!raw) return null
        const parsed = JSON.parse(raw)
        return parsed?.token ?? null
    } catch {
        return null
    }
}

api.interceptors.request.use((cfg: InternalAxiosRequestConfig) => {
    const token = readToken()
    if (token) {
        cfg.headers.set('Authorization', `Bearer ${token}`)
    }
    return cfg
})

// 401 → clear auth + redirect to login. Other errors keep the user on
// the current page so they can show inline error messages.
let _redirecting = false
function redirectToLogin() {
    if (_redirecting) return
    _redirecting = true
    localStorage.removeItem(STORAGE_KEY)
    // Hard navigation so zustand state is fully reset.
    window.location.href = '/login'
}

api.interceptors.response.use(
    (r) => r,
    (err: AxiosError<{ error?: { code: string; message: string; details?: unknown } }>) => {
        if (err.response?.status === 401) redirectToLogin()

        // Normalize the error object so callers can read .code / .message
        // consistently. We map common network errors to a friendly code.
        const envErr = err.response?.data?.error
        if (envErr) {
            ; (err as any).code = envErr.code
                ; (err as any).message = envErr.message
                ; (err as any).details = envErr.details
        } else if (err.code === 'ECONNREFUSED' || err.code === 'ERR_NETWORK') {
            // Vite proxy / dev-server couldn't reach the upstream (likely
            // the gateway or backend is not running).
            ; (err as any).code = 'UPSTREAM_UNREACHABLE'
                ; (err as any).message =
                    'Cannot reach the API gateway. Is it running on http://localhost:8080? ' +
                    'See the README for `go run ./gateway/cmd/gateway` to start it.'
        } else if (err.code === 'ECONNABORTED') {
            ; (err as any).code = 'TIMEOUT'
                ; (err as any).message = 'The request timed out. Please try again.'
        }
        return Promise.reject(err)
    },
)

// ---- Envelope helpers ----
//
// All backend responses follow this envelope (see backend/pkg/httpx):
//
//   { "success": true, "data": <payload>, "meta": <pagination?>, "error": ... }
//
// `unwrap<T>` extracts the `data` field. For paginated list endpoints
// (where `data` is an array AND `meta` is present), it additionally
// flattens `meta` into the result so callers can write:
//     unwrap<{ items: Violation[]; total: number }>(api.get('/violations'))
// and have both `.items` (the array) and `.total` (the count) work.
//
// For non-paginated calls (detail / create / login), `data` is an object
// and is returned as-is.
export function unwrap<T>(p: Promise<{
    data: { success: boolean; data: any; meta?: { page: number; page_size: number; total: number }; error?: unknown }
}>): Promise<T> {
    return p.then((r) => {
        const d = r.data.data
        const m = r.data.meta
        if (m && Array.isArray(d)) {
            // Paginated: synthesize the { items, ...meta } shape the UI expects.
            // The TS generic T is usually typed as { items: T[]; total: number; ... }
            // so this matches that contract at runtime.
            return { items: d, ...m } as unknown as T
        }
        return d as T
    })
}
