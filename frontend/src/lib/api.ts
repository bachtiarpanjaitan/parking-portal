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

// Helpers to extract the success-payload from our envelope.
export function unwrap<T>(p: Promise<{ data: { success: boolean; data: T; meta?: unknown; error?: unknown } }>): Promise<T> {
    return p.then((r) => r.data.data)
}
