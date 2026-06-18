import { useMemo, useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { api, unwrap } from '../lib/api'
import { useAuth } from '../store/auth'
import type { Invoice, SnapTokenResponse, ViolationType, InvoiceStatus } from '../types/api'

declare global {
    interface Window {
        snap?: { pay: (token: string, opts: any) => void }
    }
}

const VIOLATION_TYPES: ViolationType[] = [
    'expired_meter',
    'no_parking_zone',
    'blocking_hydrant',
    'disabled_spot',
]

const STATUS_OPTIONS: InvoiceStatus[] = ['PENDING', 'PAID', 'FAILED', 'CANCELLED']

// Human-friendly label for a violation type code.
const TYPE_LABELS: Record<ViolationType, string> = {
    expired_meter: 'Expired meter',
    no_parking_zone: 'No parking zone',
    blocking_hydrant: 'Blocking hydrant',
    disabled_spot: 'Disabled spot',
}

// Pill colors per invoice status. Tailwind needs full class names, so
// we map the status to a complete className string rather than building
// one dynamically with string concat.
const STATUS_BADGE: Record<InvoiceStatus, string> = {
    PENDING: 'bg-amber-100 text-amber-800',
    PAID: 'bg-green-100 text-green-800',
    FAILED: 'bg-red-100 text-red-800',
    CANCELLED: 'bg-slate-200 text-slate-700',
}

interface Filters {
    license_plate: string
    violation_type: ViolationType | ''
    status: InvoiceStatus | ''
    location: string
    from: string
    to: string
    page: number
    page_size: number
}

const DEFAULT_FILTERS: Filters = {
    license_plate: '',
    violation_type: '',
    status: '',
    location: '',
    from: '',
    to: '',
    page: 1,
    page_size: 10,
}

export default function InvoicesPage() {
    const user = useAuth((s) => s.user)!
    const [filters, setFilters] = useState<Filters>(DEFAULT_FILTERS)
    const [photoUrl, setPhotoUrl] = useState<string | null>(null)

    // Build the query string. We only include keys with non-empty values
    // so the URL stays clean and the backend's "ignore if empty" logic
    // does the right thing.
    const queryString = useMemo(() => {
        const params = new URLSearchParams()
        if (filters.license_plate) params.set('license_plate', filters.license_plate)
        if (filters.violation_type) params.set('violation_type', filters.violation_type)
        if (filters.status) params.set('status', filters.status)
        if (filters.location) params.set('location', filters.location)
        if (filters.from) params.set('from', new Date(filters.from).toISOString())
        if (filters.to) params.set('to', new Date(filters.to).toISOString())
        if (filters.page > 1) params.set('page', String(filters.page))
        if (filters.page_size !== DEFAULT_FILTERS.page_size) {
            params.set('page_size', String(filters.page_size))
        }
        const qs = params.toString()
        return qs ? `?${qs}` : ''
    }, [filters])

    const { data, isLoading, isFetching, error, refetch } = useQuery({
        // The queryKey includes the current filter state so React Query
        // automatically refetches when the user changes a filter.
        queryKey: ['invoices', user.id, queryString],
        queryFn: () =>
            unwrap<{ items: Invoice[]; total: number; page: number; page_size: number }>(
                api.get(`/api/v1/invoices${queryString}`),
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
                        if (
                            refreshed.status === 'PAID' ||
                            refreshed.status === 'FAILED' ||
                            refreshed.status === 'CANCELLED' ||
                            refreshed.status === 'EXPIRED'
                        ) {
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

    const updateFilter = <K extends keyof Filters>(key: K, value: Filters[K]) => {
        // Any filter change resets us to page 1 — otherwise the user
        // would be confused by a 1-item page-4 result of a search that
        // shrank the dataset.
        setFilters((f) => ({ ...f, [key]: value, page: 1 }))
    }

    const resetFilters = () => setFilters(DEFAULT_FILTERS)

    const items = data?.items ?? []
    const total = data?.total ?? 0
    const totalPages = Math.max(1, Math.ceil(total / filters.page_size))

    return (
        <div>
            <h1 className="text-2xl font-bold">My Invoices</h1>
            <p className="text-sm text-slate-500 mt-1">
                {total} invoice{total === 1 ? '' : 's'} found
            </p>

            {/* ---------- Filter bar ---------- */}
            <div className="mt-4 bg-white border rounded-lg p-4">
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-6 gap-3">
                    <div className="lg:col-span-1">
                        <label className="block text-xs font-medium text-slate-600 mb-1">
                            License plate
                        </label>
                        <input
                            type="text"
                            placeholder="e.g. B 1234 ABC"
                            className="w-full border rounded px-2 py-1.5 text-sm"
                            value={filters.license_plate}
                            onChange={(e) => updateFilter('license_plate', e.target.value)}
                        />
                    </div>
                    <div className="lg:col-span-1">
                        <label className="block text-xs font-medium text-slate-600 mb-1">
                            Violation type
                        </label>
                        <select
                            className="w-full border rounded px-2 py-1.5 text-sm"
                            value={filters.violation_type}
                            onChange={(e) =>
                                updateFilter('violation_type', e.target.value as ViolationType | '')
                            }
                        >
                            <option value="">All</option>
                            {VIOLATION_TYPES.map((t) => (
                                <option key={t} value={t}>
                                    {TYPE_LABELS[t]}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="lg:col-span-1">
                        <label className="block text-xs font-medium text-slate-600 mb-1">Status</label>
                        <select
                            className="w-full border rounded px-2 py-1.5 text-sm"
                            value={filters.status}
                            onChange={(e) =>
                                updateFilter('status', e.target.value as InvoiceStatus | '')
                            }
                        >
                            <option value="">All</option>
                            {STATUS_OPTIONS.map((s) => (
                                <option key={s} value={s}>
                                    {s}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="lg:col-span-1">
                        <label className="block text-xs font-medium text-slate-600 mb-1">
                            Location
                        </label>
                        <input
                            type="text"
                            placeholder="Street, address…"
                            className="w-full border rounded px-2 py-1.5 text-sm"
                            value={filters.location}
                            onChange={(e) => updateFilter('location', e.target.value)}
                        />
                    </div>
                    <div className="lg:col-span-1">
                        <label className="block text-xs font-medium text-slate-600 mb-1">From</label>
                        <input
                            type="date"
                            className="w-full border rounded px-2 py-1.5 text-sm"
                            value={filters.from}
                            onChange={(e) => updateFilter('from', e.target.value)}
                        />
                    </div>
                    <div className="lg:col-span-1">
                        <label className="block text-xs font-medium text-slate-600 mb-1">To</label>
                        <input
                            type="date"
                            className="w-full border rounded px-2 py-1.5 text-sm"
                            value={filters.to}
                            onChange={(e) => updateFilter('to', e.target.value)}
                        />
                    </div>
                </div>
                <div className="mt-3 flex items-center justify-between">
                    <button
                        onClick={resetFilters}
                        className="text-sm text-slate-600 hover:text-slate-900 underline"
                    >
                        Reset filters
                    </button>
                    <div className="flex items-center gap-2 text-xs text-slate-500">
                        <span>Rows per page:</span>
                        <select
                            className="border rounded px-2 py-0.5 text-xs"
                            value={filters.page_size}
                            onChange={(e) =>
                                updateFilter('page_size', Number(e.target.value))
                            }
                        >
                            {[10, 20, 50, 100].map((n) => (
                                <option key={n} value={n}>
                                    {n}
                                </option>
                            ))}
                        </select>
                    </div>
                </div>
            </div>

            {/* ---------- Loading / error states ---------- */}
            {isLoading && <p className="mt-4 text-slate-500">Loading…</p>}

            {error && (
                <div className="mt-4 rounded border border-red-200 bg-red-50 p-4 text-sm text-red-800">
                    <p className="font-semibold">Could not load invoices</p>
                    <p className="mt-1">{(error as any).message || 'Unknown error'}</p>
                </div>
            )}

            {/* ---------- Table ---------- */}
            {data && (
                <div
                    className={`mt-4 overflow-x-auto bg-white rounded-lg border ${
                        isFetching ? 'opacity-60' : ''
                    }`}
                >
                    <table className="w-full text-sm">
                        <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                            <tr>
                                <th className="px-3 py-2">When</th>
                                <th className="px-3 py-2">Plate</th>
                                <th className="px-3 py-2">Type</th>
                                <th className="px-3 py-2">Location</th>
                                <th className="px-3 py-2 text-right">Amount</th>
                                <th className="px-3 py-2">Status</th>
                                <th className="px-3 py-2 text-center">Photo</th>
                                <th className="px-3 py-2 text-right">Action</th>
                            </tr>
                        </thead>
                        <tbody>
                            {items.map((inv) => (
                                <tr key={inv.id} className="border-t hover:bg-slate-50">
                                    <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap">
                                        {new Date(inv.violation_timestamp).toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2 font-mono">{inv.license_plate}</td>
                                    <td className="px-3 py-2">{TYPE_LABELS[inv.violation_type] ?? inv.violation_type}</td>
                                    <td className="px-3 py-2 max-w-xs truncate" title={inv.location}>
                                        {inv.location}
                                    </td>
                                    <td className="px-3 py-2 text-right font-mono whitespace-nowrap">
                                        IDR {Number(inv.amount).toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2">
                                        <span
                                            className={`px-2 py-0.5 rounded text-xs font-medium ${
                                                STATUS_BADGE[inv.status]
                                            }`}
                                        >
                                            {inv.status}
                                        </span>
                                    </td>
                                    <td className="px-3 py-2 text-center">
                                        {inv.photo_url ? (
                                            <button
                                                onClick={() => setPhotoUrl(photoUrlToAbsolute(inv.photo_url))}
                                                className="text-primary-600 hover:text-primary-800 hover:underline text-xs font-medium"
                                            >
                                                View
                                            </button>
                                        ) : (
                                            <span className="text-slate-400 text-xs">—</span>
                                        )}
                                    </td>
                                    <td className="px-3 py-2 text-right whitespace-nowrap">
                                        {user.role === 'MEMBER' &&
                                            (inv.status === 'PENDING' || inv.status === 'FAILED') && (
                                                <button
                                                    onClick={() => snap.mutate(inv.id)}
                                                    disabled={snap.isPending}
                                                    className="bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white text-xs font-semibold px-3 py-1.5 rounded"
                                                >
                                                    {snap.isPending ? 'Processing…' : 'Pay'}
                                                </button>
                                            )}
                                    </td>
                                </tr>
                            ))}
                            {items.length === 0 && !isLoading && (
                                <tr>
                                    <td
                                        colSpan={8}
                                        className="px-3 py-8 text-center text-slate-500"
                                    >
                                        No invoices match your filters.
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}

            {/* ---------- Pagination ---------- */}
            {data && total > 0 && (
                <div className="mt-4 flex items-center justify-between text-sm">
                    <div className="text-slate-600">
                        Showing{' '}
                        <span className="font-semibold">
                            {(filters.page - 1) * filters.page_size + 1}
                        </span>
                        –
                        <span className="font-semibold">
                            {Math.min(filters.page * filters.page_size, total)}
                        </span>{' '}
                        of <span className="font-semibold">{total}</span>
                    </div>
                    <div className="flex items-center gap-1">
                        <PageButton
                            label="«"
                            disabled={filters.page <= 1 || isFetching}
                            onClick={() => setFilters((f) => ({ ...f, page: 1 }))}
                        />
                        <PageButton
                            label="‹"
                            disabled={filters.page <= 1 || isFetching}
                            onClick={() =>
                                setFilters((f) => ({ ...f, page: Math.max(1, f.page - 1) }))
                            }
                        />
                        <span className="px-3 text-slate-600">
                            Page <span className="font-semibold">{filters.page}</span> of{' '}
                            <span className="font-semibold">{totalPages}</span>
                        </span>
                        <PageButton
                            label="›"
                            disabled={filters.page >= totalPages || isFetching}
                            onClick={() =>
                                setFilters((f) => ({
                                    ...f,
                                    page: Math.min(totalPages, f.page + 1),
                                }))
                            }
                        />
                        <PageButton
                            label="»"
                            disabled={filters.page >= totalPages || isFetching}
                            onClick={() => setFilters((f) => ({ ...f, page: totalPages }))}
                        />
                    </div>
                </div>
            )}

            {/* ---------- Photo modal ---------- */}
            {photoUrl && (
                <PhotoModal url={photoUrl} onClose={() => setPhotoUrl(null)} />
            )}
        </div>
    )
}

function PageButton({
    label,
    disabled,
    onClick,
}: {
    label: string
    disabled?: boolean
    onClick: () => void
}) {
    return (
        <button
            onClick={onClick}
            disabled={disabled}
            className="px-2.5 py-1 rounded border bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed"
        >
            {label}
        </button>
    )
}

// Photo URLs come from the backend as relative paths like
// "/uploads/violations/<uuid>.jpg". In dev the Vite proxy / gateway will
// already route those to the right host, so the URL is usable as-is for
// the <img>. In a production build behind a different host the user may
// need to be on the same origin — that matches the current upload flow.
function photoUrlToAbsolute(url: string): string {
    if (!url) return url
    if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('data:')) {
        return url
    }
    // Prepend the API base so /uploads/* resolves against the gateway
    // (VITE_API_URL is empty in dev where the Vite proxy handles it).
    const base = (import.meta.env.VITE_API_URL || '').replace(/\/+$/, '')
    return `${base}${url.startsWith('/') ? '' : '/'}${url}`
}

function PhotoModal({ url, onClose }: { url: string; onClose: () => void }) {
    return (
        <div
            className="fixed inset-0 z-50 bg-black/70 flex items-center justify-center p-4"
            onClick={onClose}
        >
            <div
                className="relative bg-white rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-hidden"
                onClick={(e) => e.stopPropagation()}
            >
                <div className="flex items-center justify-between px-4 py-2 border-b">
                    <h2 className="text-sm font-semibold text-slate-700">Violation photo</h2>
                    <button
                        onClick={onClose}
                        className="text-slate-500 hover:text-slate-900 text-xl leading-none"
                        aria-label="Close"
                    >
                        ×
                    </button>
                </div>
                <div className="bg-slate-100 flex items-center justify-center p-4 max-h-[80vh]">
                    <img
                        src={url}
                        alt="Violation evidence"
                        className="max-w-full max-h-[75vh] object-contain"
                        onError={(e) => {
                            // If the image fails to load (e.g. demo URL
                            // doesn't exist), show a placeholder instead
                            // of a broken icon.
                            const img = e.currentTarget
                            img.style.display = 'none'
                            const parent = img.parentElement
                            if (parent && !parent.querySelector('.photo-fallback')) {
                                const fb = document.createElement('div')
                                fb.className =
                                    'photo-fallback text-slate-500 text-sm italic py-12'
                                fb.textContent = 'Photo unavailable'
                                parent.appendChild(fb)
                            }
                        }}
                    />
                </div>
            </div>
        </div>
    )
}
