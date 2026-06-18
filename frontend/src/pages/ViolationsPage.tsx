import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, unwrap } from '../lib/api'
import { useAuth } from '../store/auth'
import type { Violation, ViolationType } from '../types/api'

const VIOLATION_TYPES: ViolationType[] = [
    'expired_meter',
    'no_parking_zone',
    'blocking_hydrant',
    'disabled_spot',
]

export default function ViolationsPage() {
    const user = useAuth((s) => s.user)!
    const qc = useQueryClient()

    const { data, isLoading } = useQuery({
        queryKey: ['violations', user.role, user.id],
        queryFn: () =>
            unwrap<{ items: Violation[]; total: number }>(
                api.get(
                    '/api/v1/violations' + (user.role === 'MEMBER' ? `?member_id=${user.id}` : ''),
                ),
            ),
    })

    const create = useMutation({
        mutationFn: (body: any) => unwrap<any>(api.post('/api/v1/violations', body)),
        onSuccess: () => qc.invalidateQueries({ queryKey: ['violations'] }),
    })

    return (
        <div>
            <h1 className="text-2xl font-bold">Violations</h1>
            {user.role === 'OFFICER' && (
                <CreateForm
                    onSubmit={(b: any) => create.mutate(b)}
                    loading={create.isPending}
                    error={create.error as any}
                />
            )}

            {isLoading && <p className="mt-4 text-slate-500">Loading…</p>}

            {data && (
                <div className="mt-6 overflow-x-auto bg-white rounded-lg border">
                    <table className="w-full text-sm">
                        <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500">
                            <tr>
                                <th className="px-3 py-2">Plate</th>
                                <th className="px-3 py-2">Type</th>
                                <th className="px-3 py-2">Location</th>
                                <th className="px-3 py-2">When</th>
                                <th className="px-3 py-2 text-right">Fine</th>
                                <th className="px-3 py-2">Invoice</th>
                                <th className="px-3 py-2">Rule</th>
                            </tr>
                        </thead>
                        <tbody>
                            {((data ?? { items: [] }).items ?? []).map((v) => (
                                <tr key={v.id} className="border-t">
                                    <td className="px-3 py-2 font-mono">{v.license_plate}</td>
                                    <td className="px-3 py-2">{v.violation_type}</td>
                                    <td className="px-3 py-2">{v.location}</td>
                                    <td className="px-3 py-2 text-slate-500 text-xs">
                                        {new Date(v.violation_timestamp).toLocaleString()}
                                    </td>
                                    <td className="px-3 py-2 text-right font-mono">{v.fine_amount}</td>
                                    <td className="px-3 py-2">
                                        <span
                                            className={`px-2 py-0.5 rounded text-xs ${v.invoice_status === 'PAID'
                                                ? 'bg-green-100 text-green-800'
                                                : v.invoice_status === 'FAILED'
                                                    ? 'bg-red-100 text-red-800'
                                                    : 'bg-slate-100 text-slate-700'
                                                }`}
                                        >
                                            {v.invoice_status ?? '—'}
                                        </span>
                                    </td>
                                    <td className="px-3 py-2 text-xs text-slate-500">v{v.rule_version_number}</td>
                                </tr>
                            ))}
                            {((data ?? { items: [] }).items ?? []).length === 0 && (
                                <tr>
                                    <td colSpan={7} className="px-3 py-6 text-center text-slate-500">
                                        No violations yet.
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

function CreateForm({ onSubmit, loading, error }: any) {
    // useQuery returns an OBJECT, not an array. Destructure with `data:` rename.
    const { data: members, isLoading: loadingMembers, error: membersError } = useQuery({
        queryKey: ['members'],
        queryFn: () => unwrap<{ items: { id: string; name: string; email: string }[] }>(api.get('/api/v1/members')),
    })
    const [memberId, setMemberId] = useState('')
    const [plate, setPlate] = useState('')
    const [vtype, setVtype] = useState<ViolationType>('no_parking_zone')
    const [location, setLocation] = useState('')
    const [photoUrl, setPhotoUrl] = useState('/uploads/violations/demo.jpg')

    return (
        <form
            onSubmit={(e) => {
                e.preventDefault()
                onSubmit({
                    member_id: memberId,
                    license_plate: plate,
                    violation_type: vtype,
                    location,
                    violation_timestamp: new Date().toISOString(),
                    photo_url: photoUrl,
                })
            }}
            className="mt-4 grid grid-cols-2 md:grid-cols-6 gap-3 bg-white border rounded-lg p-4"
        >
            <select
                className="border rounded px-2 py-1 text-sm"
                value={memberId}
                onChange={(e) => setMemberId(e.target.value)}
                required
                disabled={loadingMembers}
            >
                <option value="">
                    {loadingMembers ? 'Loading members…' : '— select member —'}
                </option>
                {((members ?? { items: [] }).items ?? []).map((m) => (
                    <option key={m.id} value={m.id}>
                        {m.name} ({m.email})
                    </option>
                ))}
            </select>
            <input
                className="border rounded px-2 py-1 text-sm"
                placeholder="Plate"
                value={plate}
                onChange={(e) => setPlate(e.target.value)}
                required
            />
            <select
                className="border rounded px-2 py-1 text-sm"
                value={vtype}
                onChange={(e) => setVtype(e.target.value as ViolationType)}
            >
                {VIOLATION_TYPES.map((t) => (
                    <option key={t} value={t}>
                        {t}
                    </option>
                ))}
            </select>
            <input
                className="border rounded px-2 py-1 text-sm col-span-2"
                placeholder="Location"
                value={location}
                onChange={(e) => setLocation(e.target.value)}
                required
            />
            <button
                disabled={loading}
                className="bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white text-sm font-semibold py-1.5 rounded"
            >
                {loading ? 'Creating…' : 'Create violation'}
            </button>
            {(error || membersError) && (
                <p className="col-span-full text-sm text-red-600">
                    {(error as any)?.message || (membersError as any)?.message || 'error'}
                </p>
            )}
        </form>
    )
}
