import { useQuery } from '@tanstack/react-query'
import { api, unwrap } from '../lib/api'
import { useAuth } from '../store/auth'

interface RuleDetail {
    id: string
    violation_type: string
    base_amount: string
    day_multiplier: string
    night_multiplier: string
    repeat_0: string
    repeat_1: string
    repeat_2_plus: string
}

interface ActiveRule {
    id: string
    version_number: number
    is_active: boolean
    published_at: string
    details: RuleDetail[]
}

export default function RulesPage() {
    const user = useAuth((s) => s.user)!
    const isOfficer = user.role === 'OFFICER'

    const { data, isLoading, error } = useQuery<ActiveRule>({
        queryKey: ['rules'],
        queryFn: () => unwrap<ActiveRule>(api.get('/api/v1/rules/active')),
        enabled: isOfficer,
    })

    if (!isOfficer) {
        return <p className="text-slate-500">Only officers can view fine rules.</p>
    }
    if (isLoading) return <p className="text-slate-500">Loading…</p>
    if (error) return <p className="text-red-600">{(error as any).message || 'error'}</p>
    if (!data) return null

    return (
        <div>
            <h1 className="text-2xl font-bold">Active Fine Rule (v{data.version_number})</h1>
            <p className="text-sm text-slate-500">Published {new Date(data.published_at).toLocaleString()}</p>
            <div className="mt-6 grid grid-cols-1 md:grid-cols-2 gap-4">
                {((data ?? { details: [] }).details ?? []).map((d) => (
                    <div key={d.id} className="bg-white border rounded-lg p-4">
                        <div className="text-sm font-semibold">{d.violation_type.replace(/_/g, ' ')}</div>
                        <div className="text-3xl font-bold mt-2">IDR {Number(d.base_amount).toLocaleString()}</div>
                        <div className="mt-3 text-xs text-slate-500 space-y-1">
                            <div>
                                Day multiplier: <span className="font-mono">{d.day_multiplier}</span>
                            </div>
                            <div>
                                Night multiplier: <span className="font-mono">{d.night_multiplier}</span>
                            </div>
                            <div>
                                Repeat 0/1/2+:{' '}
                                <span className="font-mono">
                                    {d.repeat_0}/{d.repeat_1}/{d.repeat_2_plus}
                                </span>
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    )
}
