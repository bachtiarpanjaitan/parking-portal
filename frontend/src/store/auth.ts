// Auth store (zustand). Persists the JWT and user object to localStorage
// so the user stays signed in across reloads.

import { create } from 'zustand'
import { api } from '../lib/api'

export type Role = 'OFFICER' | 'MEMBER'

export interface User {
    id: string
    name: string
    email: string
    role: Role
}

interface AuthState {
    user: User | null
    token: string | null
    loading: boolean
    error: string | null
    initialized: boolean // true once we've tried to rehydrate from localStorage
    login: (email: string, password: string) => Promise<void>
    logout: () => void
    rehydrate: () => void
}

const STORAGE_KEY = 'auth-v1'

function load(): { user: User | null; token: string | null } {
    try {
        const raw = localStorage.getItem(STORAGE_KEY)
        if (!raw) return { user: null, token: null }
        const parsed = JSON.parse(raw)
        return { user: parsed?.user ?? null, token: parsed?.token ?? null }
    } catch {
        return { user: null, token: null }
    }
}

function save(user: User | null, token: string | null) {
    if (user && token) {
        localStorage.setItem(STORAGE_KEY, JSON.stringify({ user, token }))
    } else {
        localStorage.removeItem(STORAGE_KEY)
    }
}

export const useAuth = create<AuthState>((set) => {
    const initial = load()
    return {
        user: initial.user,
        token: initial.token,
        loading: false,
        error: null,
        initialized: true,

        async login(email, password) {
            set({ loading: true, error: null })
            try {
                const r = await api.post('/api/v1/auth/login', { email, password })
                const { token, user } = r.data.data
                save(user, token)
                set({ user, token, loading: false })
            } catch (e: any) {
                set({ loading: false, error: e.message || 'login failed' })
                throw e
            }
        },

        logout() {
            save(null, null)
            set({ user: null, token: null })
        },

        rehydrate() {
            const { user, token } = load()
            set({ user, token })
        },
    }
})
