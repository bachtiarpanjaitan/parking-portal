import { useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../lib/api'
import { useAuth } from '../store/auth'
import type { RuleVersion, RuleVersionWithDetails, RuleDetail, ViolationType } from '../types/api'

const VIOLATION_TYPES: ViolationType[] = [
    'expired_meter',
    'no_parking_zone',
    'blocking_hydrant',
    'disabled_spot',
]

// Human-friendly label for a violation type code.
const TYPE_LABELS: Record<ViolationType, string> = {
    expired_meter: 'Expired meter',
    no_parking_zone: 'No parking zone',
    blocking_hydrant: 'Blocking hydrant',
    disabled_spot: 'Disabled spot',
}

type StatusFilter = 'all' | 'active' | 'draft'

interface Filters {
    status: StatusFilter
    page: number
    page_size: number
}

const DEFAULT_FILTERS: Filters = {
    status: 'all',
    page: 1,
    page_size: 10,
}

// One row of the editor (a single violation type's pricing rules).
// Numbers are kept as strings while editing so partial input like
// "1." or "" doesn't snap to 0 / NaN mid-typing. We only convert to
// number on submit.
interface DetailDraft {
    violation_type: ViolationType
    base_amount: string
    day_multiplier: string
    night_multiplier: string
    repeat_0: string
    repeat_1: string
    repeat_2_plus: string
}

// Build a complete 4-row draft for "create" or pre-fill "edit" from
// an existing version. When `from` is provided we copy each row by
// violation_type, falling back to an empty row for any missing one.
function buildDraft(from?: RuleDetail[]): DetailDraft[] {
    if (from && from.length > 0) {
        return VIOLATION_TYPES.map((t) => {
            const existing = from.find((d) => d.violation_type === t)
            if (existing) {
                return {
                    violation_type: t,
                    base_amount: existing.base_amount,
                    day_multiplier: existing.day_multiplier,
                    night_multiplier: existing.night_multiplier,
                    repeat_0: existing.repeat_0,
                    repeat_1: existing.repeat_1,
                    repeat_2_plus: existing.repeat_2_plus,
                }
            }
            return { violation_type: t, ...emptyRow() }
        })
    }
    return VIOLATION_TYPES.map((t) => ({ violation_type: t, ...emptyRow() }))
}

function emptyRow(): Omit<DetailDraft, 'violation_type'> {
    return {
        base_amount: '',
        day_multiplier: '1',
        night_multiplier: '1.5',
        repeat_0: '1',
        repeat_1: '1.5',
        repeat_2_plus: '2',
    }
}

export default function RulesPage() {
    const user = useAuth((s) => s.user)!
    const isOfficer = user.role === 'OFFICER'
    const qc = useQueryClient()
    const [filters, setFilters] = useState<Filters>(DEFAULT_FILTERS)

    // editor / view modal state
    const [editing, setEditing] = useState<{
        mode: 'create' | 'edit'
        id?: string
        draft: DetailDraft[]
    } | null>(null)
    const [viewing, setViewing] = useState<RuleVersionWithDetails | null>(null)
    const [confirmDelete, setConfirmDelete] = useState<RuleVersion | null>(null)

    // Build the query string. Empty status = no filter.
    const queryString = useMemo(() => {
        const params = new URLSearchParams()
        if (filters.status !== 'all') params.set('status', filters.status)
        if (filters.page > 1) params.set('page', String(filters.page))
        if (filters.page_size !== DEFAULT_FILTERS.page_size) {
            params.set('page_size', String(filters.page_size))
        }
        const qs = params.toString()
        return qs ? `?${qs}` : ''
    }, [filters])

    const { data, isLoading, isFetching, error } = useQuery({
        queryKey: ['rules', queryString],
        queryFn: () =>
            unwrap<{ items: RuleVersion[]; total: number; page: number; page_size: number }>(
                api.get(`/api/v1/rules${queryString}`),
            ),
        enabled: isOfficer,
    })

    const create = useMutation({
        mutationFn: (body: { rules: DetailDraft[] }) =>
            unwrap<RuleVersionWithDetails>(api.post('/api/v1/rules', body)),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: ['rules'] })
            setEditing(null)
        },
        onError: (e: any) => alert(e.message || 'create failed'),
    })

    const update = useMutation({
        mutationFn: ({ id, body }: { id: string; body: { rules: DetailDraft[] } }) =>
            unwrap<RuleVersionWithDetails>(api.put(`/api/v1/rules/${id}`, body)),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: ['rules'] })
            setEditing(null)
        },
        onError: (e: any) => alert(e.message || 'update failed'),
    })

    const publish = useMutation({
        mutationFn: (id: string) =>
            unwrap<{ id: string }>(api.post(`/api/v1/rules/${id}/publish`)),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: ['rules'] })
            qc.invalidateQueries({ queryKey: ['rules', 'active'] })
        },
        onError: (e: any) => alert(e.message || 'publish failed'),
    })

    const del = useMutation({
        mutationFn: (id: string) => api.delete(`/api/v1/rules/${id}`),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: ['rules'] })
            setConfirmDelete(null)
        },
        onError: (e: any) => alert(e.message || 'delete failed'),
    })

    const openCreate = () =>
        setEditing({ mode: 'create', draft: buildDraft() })

    const openEdit = async (v: RuleVersion) => {
        try {
            const full = await unwrap<RuleVersionWithDetails>(api.get(`/api/v1/rules/${v.id}`))
            setEditing({ mode: 'edit', id: v.id, draft: buildDraft(full.details) })
        } catch (e: any) {
            alert(e.message || 'could not load rule details')
        }
    }

    const openView = async (v: RuleVersion) => {
        try {
            const full = await unwrap<RuleVersionWithDetails>(api.get(`/api/v1/rules/${v.id}`))
            setViewing(full)
        } catch (e: any) {
            alert(e.message || 'could not load rule details')
        }
    }

    const updateFilter = <K extends keyof Filters>(key: K, value: Filters[K]) => {
        setFilters((f) => ({ ...f, [key]: value, page: 1 }))
    }

    const items = data?.items ?? []
    const total = data?.total ?? 0
    const totalPages = Math.max(1, Math.ceil(total / filters.page_size))

    if (!isOfficer) {
        return <p className="text-slate-500">Only officers can view fine rules.</p>
    }

    return (
        <div>
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-2xl font-bold">Fine Rules</h1>
                    <p className="text-sm text-slate-500 mt-1">
                        {total} version{total === 1 ? '' : 's'} found
                    </p>
                </div>
                <button
                    onClick={openCreate}
                    className="bg-primary-600 hover:bg-primary-700 text-white text-sm font-semibold px-4 py-2 rounded"
                >
                    + New draft
                </button>
            </div>

            {/* ---------- Filter bar ---------- */}
            <div className="mt-4 bg-white border rounded-lg p-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                    <label className="text-xs font-medium text-slate-600">Status</label>
                    <select
                        className="border rounded px-2 py-1.5 text-sm"
                        value={filters.status}
                        onChange={(e) =>
                            updateFilter('status', e.target.value as StatusFilter)
                        }
                    >
                        <option value="all">All</option>
                        <option value="active">Active only</option>
                        <option value="draft">Drafts only</option>
                    </select>
                </div>
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

            {/* ---------- Loading / error states ---------- */}
            {isLoading && <p className="mt-4 text-slate-500">Loading…</p>}

            {error && (
                <div className="mt-4 rounded border border-red-200 bg-red-50 p-4 text-sm text-red-800">
                    <p className="font-semibold">Could not load rules</p>
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
                                <th className="px-3 py-2">Version</th>
                                <th className="px-3 py-2">Status</th>
                                <th className="px-3 py-2">Published at</th>
                                <th className="px-3 py-2">Created at</th>
                                <th className="px-3 py-2 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {items.map((v) => (
                                <tr key={v.id} className="border-t hover:bg-slate-50">
                                    <td className="px-3 py-2 font-mono font-semibold">
                                        v{v.version_number}
                                    </td>
                                    <td className="px-3 py-2">
                                        {v.is_active ? (
                                            <span className="px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">
                                                ACTIVE
                                            </span>
                                        ) : (
                                            <span className="px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-800">
                                                DRAFT
                                            </span>
                                        )}
                                    </td>
                                    <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap">
                                        {new Date(v.published_at).toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2 text-slate-500 text-xs whitespace-nowrap">
                                        {new Date(v.created_at).toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2 text-right whitespace-nowrap space-x-1">
                                        <button
                                            onClick={() => openView(v)}
                                            className="text-xs px-2 py-1 rounded border bg-white hover:bg-slate-50"
                                        >
                                            View
                                        </button>
                                        {!v.is_active && (
                                            <>
                                                <button
                                                    onClick={() => openEdit(v)}
                                                    className="text-xs px-2 py-1 rounded border bg-white hover:bg-slate-50"
                                                >
                                                    Edit
                                                </button>
                                                <button
                                                    onClick={() => publish.mutate(v.id)}
                                                    disabled={publish.isPending}
                                                    className="text-xs px-2 py-1 rounded border bg-primary-600 text-white hover:bg-primary-700 disabled:opacity-50"
                                                >
                                                    {publish.isPending
                                                        ? 'Publishing…'
                                                        : 'Publish'}
                                                </button>
                                                <button
                                                    onClick={() => setConfirmDelete(v)}
                                                    className="text-xs px-2 py-1 rounded border bg-white hover:bg-red-50 hover:border-red-300 text-red-700"
                                                >
                                                    Delete
                                                </button>
                                            </>
                                        )}
                                    </td>
                                </tr>
                            ))}
                            {items.length === 0 && !isLoading && (
                                <tr>
                                    <td colSpan={5} className="px-3 py-8 text-center text-slate-500">
                                        No rule versions match your filters.
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

            {/* ---------- Edit / Create modal ---------- */}
            {editing && (
                <EditModal
                    id={editing.id}
                    mode={editing.mode}
                    draft={editing.draft}
                    onChange={setEditing}
                    onSubmit={() => {
                        if (editing.mode === 'create') {
                            create.mutate({ rules: editing.draft })
                        } else if (editing.id) {
                            update.mutate({ id: editing.id, body: { rules: editing.draft } })
                        }
                    }}
                    pending={create.isPending || update.isPending}
                />
            )}

            {/* ---------- View modal ---------- */}
            {viewing && <ViewModal version={viewing} onClose={() => setViewing(null)} />}

            {/* ---------- Confirm delete modal ---------- */}
            {confirmDelete && (
                <ConfirmDeleteModal
                    version={confirmDelete}
                    pending={del.isPending}
                    onCancel={() => setConfirmDelete(null)}
                    onConfirm={() => del.mutate(confirmDelete.id)}
                />
            )}
        </div>
    )
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

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

// EditModal is used for both Create and Edit. The parent owns the draft
// state so the existing form fields are preserved if the user cancels
// and re-opens.
function EditModal({
    mode,
    id,
    draft,
    onChange,
    onSubmit,
    pending,
}: {
    mode: 'create' | 'edit'
    id?: string
    draft: DetailDraft[]
    onChange: (next: { mode: 'create' | 'edit'; id?: string; draft: DetailDraft[] } | null) => void
    onSubmit: () => void
    pending: boolean
}) {
    const setRow = (idx: number, patch: Partial<DetailDraft>) => {
        const next = draft.map((d, i) => (i === idx ? { ...d, ...patch } : d))
        onChange({ mode, id, draft: next })
    }
    return (
        <div
            className="fixed inset-0 z-50 bg-black/70 flex items-center justify-center p-4"
            onClick={() => onChange(null)}
        >
            <div
                className="relative bg-white rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] overflow-hidden"
                onClick={(e) => e.stopPropagation()}
            >
                <div className="flex items-center justify-between px-4 py-2 border-b">
                    <h2 className="text-sm font-semibold text-slate-700">
                        {mode === 'create' ? 'New rule draft' : 'Edit rule draft'}
                    </h2>
                    <button
                        onClick={() => onChange(null)}
                        className="text-slate-500 hover:text-slate-900 text-xl leading-none"
                        aria-label="Close"
                    >
                        ×
                    </button>
                </div>
                <div className="p-4 overflow-auto max-h-[75vh]">
                    <p className="text-xs text-slate-500 mb-3">
                        Set the base amount and multipliers for each of the 4 violation
                        types. All numbers must be &gt; 0. Click <em>Save</em> to create
                        (or update) the draft; it will not take effect until an officer
                        publishes it.
                    </p>
                    <table className="w-full text-sm">
                        <thead className="text-left text-xs uppercase text-slate-500 border-b">
                            <tr>
                                <th className="px-2 py-2">Violation type</th>
                                <th className="px-2 py-2">Base (IDR)</th>
                                <th className="px-2 py-2">Day ×</th>
                                <th className="px-2 py-2">Night ×</th>
                                <th className="px-2 py-2">Repeat 0 ×</th>
                                <th className="px-2 py-2">Repeat 1 ×</th>
                                <th className="px-2 py-2">Repeat 2+ ×</th>
                            </tr>
                        </thead>
                        <tbody>
                            {draft.map((d, idx) => (
                                <tr key={d.violation_type} className="border-b">
                                    <td className="px-2 py-2 font-medium">
                                        {TYPE_LABELS[d.violation_type]}
                                    </td>
                                    <td className="px-2 py-2">
                                        <input
                                            type="number"
                                            min="0"
                                            step="any"
                                            className="w-28 border rounded px-2 py-1 text-sm font-mono"
                                            value={d.base_amount}
                                            onChange={(e) =>
                                                setRow(idx, { base_amount: e.target.value })
                                            }
                                        />
                                    </td>
                                    <td className="px-2 py-2">
                                        <NumberInput
                                            value={d.day_multiplier}
                                            onChange={(v) => setRow(idx, { day_multiplier: v })}
                                        />
                                    </td>
                                    <td className="px-2 py-2">
                                        <NumberInput
                                            value={d.night_multiplier}
                                            onChange={(v) =>
                                                setRow(idx, { night_multiplier: v })
                                            }
                                        />
                                    </td>
                                    <td className="px-2 py-2">
                                        <NumberInput
                                            value={d.repeat_0}
                                            onChange={(v) => setRow(idx, { repeat_0: v })}
                                        />
                                    </td>
                                    <td className="px-2 py-2">
                                        <NumberInput
                                            value={d.repeat_1}
                                            onChange={(v) => setRow(idx, { repeat_1: v })}
                                        />
                                    </td>
                                    <td className="px-2 py-2">
                                        <NumberInput
                                            value={d.repeat_2_plus}
                                            onChange={(v) => setRow(idx, { repeat_2_plus: v })}
                                        />
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
                <div className="flex items-center justify-end gap-2 px-4 py-3 border-t bg-slate-50">
                    <button
                        onClick={() => onChange(null)}
                        className="text-sm px-3 py-1.5 rounded border bg-white hover:bg-slate-50"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={onSubmit}
                        disabled={pending}
                        className="text-sm px-3 py-1.5 rounded bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white font-semibold"
                    >
                        {pending ? 'Saving…' : mode === 'create' ? 'Create draft' : 'Save changes'}
                    </button>
                </div>
            </div>
        </div>
    )
}

function NumberInput({
    value,
    onChange,
}: {
    value: string
    onChange: (v: string) => void
}) {
    return (
        <input
            type="number"
            min="0"
            step="any"
            className="w-20 border rounded px-2 py-1 text-sm font-mono"
            value={value}
            onChange={(e) => onChange(e.target.value)}
        />
    )
}

function ViewModal({
    version,
    onClose,
}: {
    version: RuleVersionWithDetails
    onClose: () => void
}) {
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
                    <h2 className="text-sm font-semibold text-slate-700">
                        Rule v{version.version_number}{' '}
                        {version.is_active ? (
                            <span className="ml-2 px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">
                                ACTIVE
                            </span>
                        ) : (
                            <span className="ml-2 px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-800">
                                DRAFT
                            </span>
                        )}
                    </h2>
                    <button
                        onClick={onClose}
                        className="text-slate-500 hover:text-slate-900 text-xl leading-none"
                        aria-label="Close"
                    >
                        ×
                    </button>
                </div>
                <div className="p-4 overflow-auto max-h-[80vh]">
                    <p className="text-xs text-slate-500 mb-3">
                        Published {new Date(version.published_at).toLocaleString()} · Created{' '}
                        {new Date(version.created_at).toLocaleString()}
                    </p>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                        {version.details.map((d) => (
                            <div key={d.id} className="bg-slate-50 border rounded-lg p-4">
                                <div className="text-sm font-semibold">
                                    {TYPE_LABELS[d.violation_type as ViolationType] ??
                                        d.violation_type}
                                </div>
                                <div className="text-2xl font-bold mt-1">
                                    IDR {Number(d.base_amount).toLocaleString()}
                                </div>
                                <div className="mt-3 text-xs text-slate-600 space-y-1 font-mono">
                                    <div>Day ×: {d.day_multiplier}</div>
                                    <div>Night ×: {d.night_multiplier}</div>
                                    <div>
                                        Repeat 0 / 1 / 2+ ×: {d.repeat_0} / {d.repeat_1} /{' '}
                                        {d.repeat_2_plus}
                                    </div>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    )
}

function ConfirmDeleteModal({
    version,
    pending,
    onCancel,
    onConfirm,
}: {
    version: RuleVersion
    pending: boolean
    onCancel: () => void
    onConfirm: () => void
}) {
    return (
        <div
            className="fixed inset-0 z-50 bg-black/70 flex items-center justify-center p-4"
            onClick={onCancel}
        >
            <div
                className="relative bg-white rounded-lg shadow-xl max-w-md w-full overflow-hidden"
                onClick={(e) => e.stopPropagation()}
            >
                <div className="p-5">
                    <h2 className="text-base font-semibold text-slate-900">
                        Delete rule v{version.version_number}?
                    </h2>
                    <p className="text-sm text-slate-600 mt-2">
                        This permanently removes the draft version and its 4 detail rows.
                        The currently active version is unaffected.
                    </p>
                </div>
                <div className="flex items-center justify-end gap-2 px-5 py-3 border-t bg-slate-50">
                    <button
                        onClick={onCancel}
                        className="text-sm px-3 py-1.5 rounded border bg-white hover:bg-slate-50"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={onConfirm}
                        disabled={pending}
                        className="text-sm px-3 py-1.5 rounded bg-red-600 hover:bg-red-700 disabled:opacity-50 text-white font-semibold"
                    >
                        {pending ? 'Deleting…' : 'Delete'}
                    </button>
                </div>
            </div>
        </div>
    )
}
