import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, unwrap } from '../lib/api'
import { useAuth } from '../store/auth'
import type { HistoryEntry, ViolationType, InvoiceStatus } from '../types/api'

const VIOLATION_TYPES: ViolationType[] = [
    'expired_meter',
    'no_parking_zone',
    'blocking_hydrant',
    'disabled_spot',
]

const INVOICE_STATUS_OPTIONS: InvoiceStatus[] = ['PENDING', 'PAID', 'FAILED', 'CANCELLED']
// Payment statuses include EXPIRED in addition to the invoice statuses.
const PAYMENT_STATUS_OPTIONS = ['PENDING', 'PAID', 'FAILED', 'CANCELLED', 'EXPIRED'] as const

// Human-friendly label for a violation type code.
const TYPE_LABELS: Record<ViolationType, string> = {
    expired_meter: 'Expired meter',
    no_parking_zone: 'No parking zone',
    blocking_hydrant: 'Blocking hydrant',
    disabled_spot: 'Disabled spot',
}

// Tailwind needs full class names; we map each status to a complete
// className string rather than building one dynamically with string concat.
const STATUS_BADGE: Record<string, string> = {
    PENDING: 'bg-amber-100 text-amber-800',
    PAID: 'bg-green-100 text-green-800',
    FAILED: 'bg-red-100 text-red-800',
    CANCELLED: 'bg-slate-200 text-slate-700',
    EXPIRED: 'bg-slate-200 text-slate-700',
}

interface Filters {
    license_plate: string
    violation_type: ViolationType | ''
    location: string
    invoice_status: InvoiceStatus | ''
    payment_status: string
    from: string
    to: string
    page: number
    page_size: number
}

const DEFAULT_FILTERS: Filters = {
    license_plate: '',
    violation_type: '',
    location: '',
    invoice_status: '',
    payment_status: '',
    from: '',
    to: '',
    page: 1,
    page_size: 10,
}

export default function HistoryPage() {
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
        if (filters.location) params.set('location', filters.location)
        if (filters.invoice_status) params.set('invoice_status', filters.invoice_status)
        if (filters.payment_status) params.set('payment_status', filters.payment_status)
        if (filters.from) params.set('from', new Date(filters.from).toISOString())
        if (filters.to) params.set('to', new Date(filters.to).toISOString())
        if (filters.page > 1) params.set('page', String(filters.page))
        if (filters.page_size !== DEFAULT_FILTERS.page_size) {
            params.set('page_size', String(filters.page_size))
        }
        const qs = params.toString()
        return qs ? `?${qs}` : ''
    }, [filters])

    const { data, isLoading, isFetching, error } = useQuery({
        // The queryKey includes the current filter state so React Query
        // automatically refetches when the user changes a filter.
        queryKey: ['history', user.id, queryString],
        queryFn: () =>
            unwrap<{ items: HistoryEntry[]; total: number; page: number; page_size: number }>(
                api.get(`/api/v1/history${queryString}`),
            ),
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
            <h1 className="text-2xl font-bold">My History</h1>
            <p className="text-sm text-slate-500 mt-1">
                {total} record{total === 1 ? '' : 's'} found
            </p>

            {/* ---------- Filter bar ---------- */}
            <div className="mt-4 bg-white border rounded-lg p-4">
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-7 gap-3">
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
                                updateFilter(
                                    'violation_type',
                                    e.target.value as ViolationType | '',
                                )
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
                        <label className="block text-xs font-medium text-slate-600 mb-1">
                            Invoice status
                        </label>
                        <select
                            className="w-full border rounded px-2 py-1.5 text-sm"
                            value={filters.invoice_status}
                            onChange={(e) =>
                                updateFilter(
                                    'invoice_status',
                                    e.target.value as InvoiceStatus | '',
                                )
                            }
                        >
                            <option value="">All</option>
                            {INVOICE_STATUS_OPTIONS.map((s) => (
                                <option key={s} value={s}>
                                    {s}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="lg:col-span-1">
                        <label className="block text-xs font-medium text-slate-600 mb-1">
                            Payment status
                        </label>
                        <select
                            className="w-full border rounded px-2 py-1.5 text-sm"
                            value={filters.payment_status}
                            onChange={(e) => updateFilter('payment_status', e.target.value)}
                        >
                            <option value="">All</option>
                            {PAYMENT_STATUS_OPTIONS.map((s) => (
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
                            onChange={(e) => updateFilter('page_size', Number(e.target.value))}
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
                    <p className="font-semibold">Could not load history</p>
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
                                <th className="px-3 py-2 text-right">Fine</th>
                                <th className="px-3 py-2">Rule v</th>
                                <th className="px-3 py-2">Invoice</th>
                                <th className="px-3 py-2">Payment</th>
                                <th className="px-3 py-2 text-center">Photo</th>
                            </tr>
                        </thead>
                        <tbody>
                            {items.map((h) => (
                                <tr key={h.violation_id} className="border-t hover:bg-slate-50">
                                    <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap">
                                        {new Date(h.violation_timestamp).toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2 font-mono">{h.license_plate}</td>
                                    <td className="px-3 py-2">
                                        {TYPE_LABELS[h.violation_type as ViolationType] ??
                                            h.violation_type}
                                    </td>
                                    <td
                                        className="px-3 py-2 max-w-xs truncate"
                                        title={h.location}
                                    >
                                        {h.location}
                                    </td>
                                    <td className="px-3 py-2 text-right font-mono whitespace-nowrap">
                                        IDR {h.fine_amount.toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2 text-xs">v{h.rule_version_number}</td>
                                    <td className="px-3 py-2">
                                        {h.invoice_status ? (
                                            <span
                                                className={`px-2 py-0.5 rounded text-xs font-medium ${
                                                    STATUS_BADGE[h.invoice_status] ??
                                                    'bg-slate-100 text-slate-700'
                                                }`}
                                            >
                                                {h.invoice_status}
                                            </span>
                                        ) : (
                                            <span className="text-slate-400 text-xs">—</span>
                                        )}
                                    </td>
                                    <td className="px-3 py-2">
                                        {h.payment_status ? (
                                            <span
                                                className={`px-2 py-0.5 rounded text-xs font-medium ${
                                                    STATUS_BADGE[h.payment_status] ??
                                                    'bg-slate-100 text-slate-700'
                                                }`}
                                            >
                                                {h.payment_status}
                                            </span>
                                        ) : (
                                            <span className="text-slate-400 text-xs">—</span>
                                        )}
                                    </td>
                                    <td className="px-3 py-2 text-center">
                                        {h.photo_url ? (
                                            <button
                                                onClick={() =>
                                                    setPhotoUrl(photoUrlToAbsolute(h.photo_url))
                                                }
                                                className="text-primary-600 hover:text-primary-800 hover:underline text-xs font-medium"
                                            >
                                                View
                                            </button>
                                        ) : (
                                            <span className="text-slate-400 text-xs">—</span>
                                        )}
                                    </td>
                                </tr>
                            ))}
                            {items.length === 0 && !isLoading && (
                                <tr>
                                    <td
                                        colSpan={9}
                                        className="px-3 py-8 text-center text-slate-500"
                                    >
                                        No history records match your filters.
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
            {photoUrl && <PhotoModal url={photoUrl} onClose={() => setPhotoUrl(null)} />}
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
// the <img>. We prepend VITE_API_URL when set (production builds behind
// a different host).
function photoUrlToAbsolute(url: string): string {
    if (!url) return url
    if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('data:')) {
        return url
    }
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
