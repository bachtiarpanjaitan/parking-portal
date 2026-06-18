import { useQuery, useMutation } from '@tanstack/react-query'
import { api, unwrap } from '../lib/api'
import { useAuth } from '../store/auth'
import type { Invoice, SnapTokenResponse } from '../types/api'

declare global {
    interface Window {
        snap?: { pay: (token: string, opts: any) => void }
    }
}

export default function InvoicesPage() {
    const user = useAuth((s) => s.user)!

    const { data, isLoading, refetch } = useQuery({
        queryKey: ['invoices', user.id],
        queryFn: () =>
            unwrap<{ items: Invoice[]; total: number }>(
                api.get('/api/v1/invoices' + (user.role === 'MEMBER' ? `?member_id=${user.id}` : '')),
            ),
    })

    const snap = useMutation({
        mutationFn: (invoice_id: string) =>
            unwrap<SnapTokenResponse>(api.post('/api/v1/payments/snap-token', { invoice_id })),
        onSuccess: (resp) => {
            if (!window.snap) {
                alert('Midtrans Snap.js not loaded. Check your network.')
                return
            }
            window.snap.pay(resp.snap_token, {
                onSuccess: () => {
                    alert('Payment success!')
                    refetch()
                },
                onPending: () => {
                    alert('Payment pending…')
                },
                onError: () => {
                    alert('Payment error')
                },
                onClose: () => {
                    /* user closed popup */
                },
            })
        },
        onError: (e: any) => alert(e.message || 'snap-token failed'),
    })

    return (
        <div>
            <h1 className="text-2xl font-bold">Invoices</h1>
            {isLoading && <p className="mt-4 text-slate-500">Loading…</p>}

            {data && (
                <div className="mt-6 space-y-3">
                    {((data ?? { items: [] }).items ?? []).map((i) => (
                        <div
                            key={i.id}
                            className="bg-white border rounded-lg p-4 flex items-center justify-between"
                        >
                            <div>
                                <div className="font-mono text-xs text-slate-500">{i.id}</div>
                                <div className="text-2xl font-semibold mt-1">
                                    IDR {Number(i.amount).toLocaleString()}
                                </div>
                                <div className="text-xs text-slate-500 mt-1">Status: {i.status}</div>
                            </div>
                            {user.role === 'MEMBER' && (i.status === 'PENDING' || i.status === 'FAILED') && (
                                <button
                                    onClick={() => snap.mutate(i.id)}
                                    disabled={snap.isPending}
                                    className="bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white text-sm font-semibold px-4 py-2 rounded"
                                >
                                    {snap.isPending ? 'Preparing…' : 'Pay with Midtrans'}
                                </button>
                            )}
                        </div>
                    ))}
                    {((data ?? { items: [] }).items ?? []).length === 0 && <p className="text-slate-500">No invoices.</p>}
                </div>
            )}
        </div>
    )
}
