import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../store/auth'

export default function LoginPage() {
    const login = useAuth((s) => s.login)
    const loading = useAuth((s) => s.loading)
    const error = useAuth((s) => s.error)
    const [email, setEmail] = useState('officer@example.com')
    const [password, setPassword] = useState('password123')
    const nav = useNavigate()
    const [params] = useSearchParams()
    const next = params.get('next') || '/'

    async function onSubmit(e: React.FormEvent) {
        e.preventDefault()
        try {
            await login(email, password)
            nav(next, { replace: true })
        } catch {
            // error already in store
        }
    }

    return (
        <div className="min-h-full flex items-center justify-center p-6 bg-gradient-to-br from-primary-50 to-slate-100">
            <form
                onSubmit={onSubmit}
                className="w-full max-w-sm bg-white rounded-xl shadow-lg p-8 space-y-5"
            >
                <div>
                    <h1 className="text-2xl font-bold text-slate-900">Parking Violation Portal</h1>
                    <p className="text-sm text-slate-500 mt-1">Sign in to continue</p>
                </div>

                <label className="block">
                    <span className="text-sm font-medium text-slate-700">Email</span>
                    <input
                        type="email"
                        required
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        className="mt-1 w-full rounded border-slate-300 border px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
                    />
                </label>
                <label className="block">
                    <span className="text-sm font-medium text-slate-700">Password</span>
                    <input
                        type="password"
                        required
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        className="mt-1 w-full rounded border-slate-300 border px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
                    />
                </label>

                {error && <p className="text-sm text-red-600">{error}</p>}

                <button
                    type="submit"
                    disabled={loading}
                    className="w-full bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white font-semibold py-2 rounded"
                >
                    {loading ? 'Signing in…' : 'Sign in'}
                </button>

                <details className="text-xs text-slate-500">
                    <summary className="cursor-pointer">Demo accounts</summary>
                    <ul className="mt-2 space-y-1 font-mono">
                        <li>officer@example.com / password123 (OFFICER)</li>
                        <li>member@example.com / password123 (MEMBER)</li>
                        <li>member2@example.com / password123 (MEMBER)</li>
                    </ul>
                </details>
            </form>
        </div>
    )
}
