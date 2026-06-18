import { useEffect } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { useAuth, type Role } from './store/auth'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import ViolationsPage from './pages/ViolationsPage'
import InvoicesPage from './pages/InvoicesPage'
import HistoryPage from './pages/HistoryPage'
import RulesPage from './pages/RulesPage'
import Layout from './components/Layout'

function RequireAuth({ children }: { children: React.ReactNode }) {
    const user = useAuth((s) => s.user)
    const loc = useLocation()
    if (!user) {
        return <Navigate to={`/login?next=${encodeURIComponent(loc.pathname)}`} replace />
    }
    return <Layout>{children}</Layout>
}

function RoleGuard({ roles, children }: { roles: Role[]; children: React.ReactNode }) {
    const user = useAuth((s) => s.user)!
    if (!roles.includes(user.role)) {
        return <Navigate to="/" replace />
    }
    return <>{children}</>
}

export default function App() {
    // Rehydrate the auth state from localStorage on first render. The store
    // already loads it lazily via its initializer, but calling rehydrate()
    // explicitly makes the intent obvious and gives us a hook for future
    // server-side validation.
    const rehydrate = useAuth((s) => s.rehydrate)
    useEffect(() => {
        rehydrate()
    }, [rehydrate])

    return (
        <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route
                path="/"
                element={
                    <RequireAuth>
                        <DashboardPage />
                    </RequireAuth>
                }
            />
            <Route
                path="/violations"
                element={
                    <RequireAuth>
                        <ViolationsPage />
                    </RequireAuth>
                }
            />
            <Route
                path="/invoices"
                element={
                    <RequireAuth>
                        <InvoicesPage />
                    </RequireAuth>
                }
            />
            <Route
                path="/history"
                element={
                    <RequireAuth>
                        <HistoryPage />
                    </RequireAuth>
                }
            />
            <Route
                path="/rules"
                element={
                    <RequireAuth>
                        <RoleGuard roles={['OFFICER']}>
                            <RulesPage />
                        </RoleGuard>
                    </RequireAuth>
                }
            />
            <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
    )
}
