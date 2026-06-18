import { useQuery } from '@tanstack/react-query'
import { api, unwrap } from '../lib/api'
import { useAuth } from '../store/auth'

export default function DashboardPage() {
    const user = useAuth((s) => s.user)!

    // Officer sees a quick KPI; member sees their own counts.
    const { data: counts, isLoading } = useQuery({
        queryKey: ['dashboard', user.role, user.id],
        queryFn: async () => {
            const [violations, invoices] = await Promise.all([
                unwrap<{ items: any[]; total: number }>(
                    api.get('/api/v1/violations' + (user.role === 'MEMBER' ? `?member_id=${user.id}` : '')),
                ),
                unwrap<{ items: any[]; total: number }>(
                    api.get('/api/v1/invoices' + (user.role === 'MEMBER' ? `?member_id=${user.id}` : '')),
                ),
            ])
            return {
                violations: violations.total,
                invoices: invoices.total,
                pendingInvoices: invoices.items.filter((i: any) => i.status === 'PENDING').length,
            }
        },
    })

    return (
        <div>
            <h1 className="text-2xl font-bold text-slate-900">Welcome, {user.name}</h1>
            <p className="text-sm text-slate-500 mt-1">
                Role: <span className="font-mono">{user.role}</span>
            </p>

            {isLoading && <p className="mt-6 text-slate-500">Loading…</p>}

            {counts && (
                <div className="mt-6 grid grid-cols-1 md:grid-cols-3 gap-4">
                    <Card label="Violations" value={counts.violations} />
                    <Card label="Invoices" value={counts.invoices} />
                    <Card label="Pending invoices" value={counts.pendingInvoices} accent />
                </div>
            )}
        </div>
    )
}

function Card({ label, value, accent }: { label: string; value: number; accent?: boolean }) {
    return (
        <div
            className={`rounded-lg border p-5 ${accent ? 'border-amber-300 bg-amber-50' : 'border-slate-200 bg-white'
                }`}
        >
            <div className="text-xs uppercase tracking-wide text-slate-500">{label}</div>
            <div className="text-3xl font-bold mt-2">{value}</div>
        </div>
    )
}
