import { useQuery } from '@tanstack/react-query'
import { api, unwrap } from '../lib/api'
import { useAuth } from '../store/auth'
import type { HistoryEntry } from '../types/api'

export default function HistoryPage() {
    const user = useAuth((s) => s.user)!

    const { data, isLoading, error } = useQuery({
        queryKey: ['history', user.id],
        queryFn: () =>
            unwrap<{ items: HistoryEntry[]; total: number }>(
                api.get('/api/v1/history' + (user.role === 'MEMBER' ? `?member_id=${user.id}` : '')),
            ),
    })

    return (
        <div>
            <h1 className="text-2xl font-bold">History</h1>

            {isLoading && <p className="mt-4 text-slate-500">Loading…</p>}

            {error && (
                <div className="mt-4 rounded border border-red-200 bg-red-50 p-4 text-sm text-red-800">
                    <p className="font-semibold">Could not load history</p>
                    <p className="mt-1">{(error as any).message || 'Unknown error'}</p>
                </div>
            )}

            {data && (
                <div className="mt-6 overflow-x-auto bg-white rounded-lg border">
                    <table className="w-full text-sm">
                        <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                            <tr>
                                <th className="px-3 py-2">When</th>
                                <th className="px-3 py-2">Plate</th>
                                <th className="px-3 py-2">Type</th>
                                <th className="px-3 py-2">Location</th>
                                <th className="px-3 py-2 text-right">Fine</th>
                                <th className="px-3 py-2">Rule v</th>
                                <th className="px-3 py-2">Invoice</th>
                                <th className="px-3 py-2">Payment</th>
                            </tr>
                        </thead>
                        <tbody>
                            {((data ?? { items: [] }).items ?? []).map((h) => (
                                <tr key={h.violation_id} className="border-t">
                                    <td className="px-3 py-2 text-slate-500 text-xs">
                                        {new Date(h.violation_timestamp).toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2 font-mono">{h.license_plate}</td>
                                    <td className="px-3 py-2">{h.violation_type}</td>
                                    <td className="px-3 py-2">{h.location}</td>
                                    <td className="px-3 py-2 text-right font-mono">
                                        {h.fine_amount.toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2 text-xs">v{h.rule_version_number}</td>
                                    <td className="px-3 py-2">{h.invoice_status}</td>
                                    <td className="px-3 py-2">{h.payment_status || '—'}</td>
                                </tr>
                            ))}
                            {((data ?? { items: [] }).items ?? []).length === 0 && (
                                <tr>
                                    <td colSpan={8} className="px-3 py-6 text-center text-slate-500">
                                        No history yet.
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    )
}
