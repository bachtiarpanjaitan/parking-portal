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
            if (!resp.snap_token) {
                // No snap token = already PAID → just refetch
                refetch()
                return
            }
            if (!window.snap) {
                alert('Midtrans Snap.js not loaded. Check your network.')
                return
            }
            // Polling fallback: Snap.js occasionally fails to decode a
            // mock / invalid token and never fires any of the callbacks
            // below (no onError, no onClose). Start a short-lived poller
            // that hits /refresh every 2s for up to 30s so the UI is
            // always reconciled with Midtrans, even if Snap.js goes silent.
            const POLL_INTERVAL_MS = 2_000
            const POLL_TIMEOUT_MS = 30_000
            const pollUntilTerminal = async (): Promise<void> => {
                const deadline = Date.now() + POLL_TIMEOUT_MS
                while (Date.now() < deadline) {
                    await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS))
                    try {
                        const refreshed = await unwrap<{ status: string }>(
                            api.post(`/api/v1/payments/${resp.payment_id}/refresh`),
                        )
                        if (refreshed.status === 'PAID' || refreshed.status === 'FAILED' ||
                            refreshed.status === 'CANCELLED' || refreshed.status === 'EXPIRED') {
                            await refetch()
                            return
                        }
                    } catch (e: any) {
                        console.error('poll refresh failed:', e)
                    }
                }
                // Timed out — still refetch so the UI shows the latest known state.
                await refetch()
            }

            const refreshNow = async () => {
                try {
                    await unwrap(api.post(`/api/v1/payments/${resp.payment_id}/refresh`))
                } catch (e: any) {
                    console.error('refresh failed:', e)
                }
                await refetch()
            }

            // Fire the poller in parallel with the Snap.js callbacks.
            // Whichever path finishes first wins; the other becomes a no-op.
            void pollUntilTerminal()

            window.snap.pay(resp.snap_token, {
                onSuccess: refreshNow,
                onPending: refreshNow,
                onError: refreshNow,
                onClose: refreshNow,
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
                                    {snap.isPending ? 'Processing…' : 'Pay / Check Status'}
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
