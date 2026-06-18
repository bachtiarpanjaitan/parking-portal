// Shared API types (subset of the full backend response).

export type Role = 'OFFICER' | 'MEMBER'
export type InvoiceStatus = 'PENDING' | 'PAID' | 'FAILED' | 'CANCELLED'
export type ViolationType = 'expired_meter' | 'no_parking_zone' | 'blocking_hydrant' | 'disabled_spot'

export interface User {
    id: string
    name: string
    email: string
    role: Role
}

export interface Violation {
    id: string
    member_id: string
    rule_version_id: string
    rule_version_number: number
    license_plate: string
    violation_type: ViolationType
    location: string
    violation_timestamp: string
    fine_amount: string
    photo_url: string
    invoice_id?: string
    invoice_status?: InvoiceStatus
    calculation_snapshot: any
    created_at: string
}

// Invoice is the row shape returned by GET /invoices (list). It embeds the
// joined violation fields so the UI can render the table without a
// second round-trip per row (no N+1).
//
// `latest_payment` is only present on the detail endpoint
// (GET /invoices/:id) — it is omitted on the list response.
export interface Invoice {
    id: string
    violation_id: string
    member_id: string
    amount: string
    status: InvoiceStatus
    created_at: string
    updated_at: string
    // --- joined violation fields (list only) ---
    license_plate: string
    violation_type: ViolationType
    location: string
    violation_timestamp: string
    photo_url: string
    rule_version_number?: number
    // --- detail-only ---
    latest_payment?: { id: string; status: string; scenario?: string; created_at: string } | null
}

export interface HistoryEntry {
    violation_id: string
    member_id: string
    license_plate: string
    violation_type: ViolationType
    location: string
    violation_timestamp: string
    fine_amount: number
    photo_url: string
    rule_version_id: string
    rule_version_number: number
    invoice_id: string
    invoice_status: InvoiceStatus
    payment_status?: string
    payment_tx_status?: string  // Midtrans transaction_status (capture/settlement/pending/...)
    calculation_snapshot: any
}

// Fine rule module shapes. See .ai/DATABASE_MAPPING.md → fine_rule_versions
// + fine_rule_details. A Version is the header row; Details are the
// 4 child rows (one per violation_type). Multipliers are stored as
// strings because they come from NUMERIC(3,2) in the DB.
export interface RuleDetail {
    id: string
    rule_version_id: string
    violation_type: ViolationType
    base_amount: string
    day_multiplier: string
    night_multiplier: string
    repeat_0: string
    repeat_1: string
    repeat_2_plus: string
    created_at: string
    updated_at: string
}

export interface RuleVersion {
    id: string
    version_number: number
    is_active: boolean
    published_at: string
    created_by: string
    created_at: string
    updated_at: string
}

export interface RuleVersionWithDetails extends RuleVersion {
    details: RuleDetail[]
}

export interface SnapTokenResponse {
    payment_id: string
    order_id: string
    snap_token: string
    redirect_url: string
}
