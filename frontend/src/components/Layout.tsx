import { NavLink, useNavigate } from 'react-router-dom'
import { useAuth } from '../store/auth'

const navByRole = {
    OFFICER: [
        { to: '/', label: 'Dashboard' },
        { to: '/violations', label: 'Violations' },
        { to: '/invoices', label: 'Invoices' },
        { to: '/history', label: 'History' },
        { to: '/rules', label: 'Fine Rules' },
    ],
    MEMBER: [
        { to: '/', label: 'Dashboard' },
        { to: '/invoices', label: 'My Invoices' },
        { to: '/history', label: 'My History' },
    ],
} as const

export default function Layout({ children }: { children: React.ReactNode }) {
    const user = useAuth((s) => s.user)
    const logout = useAuth((s) => s.logout)
    const nav = useNavigate()

    // Layout is only rendered when user is non-null (RequireAuth guards it).
    // We assert non-null here for the type-narrowing convenience.
    if (!user) return null
    const items = navByRole[user.role]

    return (
        <div className="flex h-full">
            <aside className="w-60 bg-primary-700 text-white flex flex-col">
                <div className="px-6 py-5 border-b border-primary-600">
                    <div className="text-lg font-semibold">Parking Portal</div>
                    <div className="text-xs text-primary-200 mt-1">{user.role}</div>
                </div>
                <nav className="flex-1 px-3 py-4 space-y-1">
                    {items.map((i) => (
                        <NavLink
                            key={i.to}
                            to={i.to}
                            end
                            className={({ isActive }) =>
                                `block px-3 py-2 rounded text-sm ${isActive ? 'bg-primary-900 font-semibold' : 'hover:bg-primary-600'
                                }`
                            }
                        >
                            {i.label}
                        </NavLink>
                    ))}
                </nav>
                <div className="p-3 border-t border-primary-600 text-xs">
                    <div className="mb-2">
                        <div className="font-semibold">{user.name}</div>
                        <div className="text-primary-200">{user.email}</div>
                    </div>
                    <button
                        onClick={() => {
                            logout()
                            nav('/login', { replace: true })
                        }}
                        className="w-full bg-primary-900 hover:bg-black py-1.5 rounded"
                    >
                        Sign out
                    </button>
                </div>
            </aside>
            <main className="flex-1 overflow-auto">
                <div className="p-6">{children}</div>
            </main>
        </div>
    )
}
