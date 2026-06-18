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

export interface Invoice {
    id: string
    violation_id: string
    member_id: string
    amount: string
    status: InvoiceStatus
    created_at: string
    updated_at: string
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
    rule_version_id: string
    rule_version_number: number
    invoice_id: string
    invoice_status: InvoiceStatus
    payment_status?: string
    payment_tx_status?: string  // Midtrans transaction_status (capture/settlement/pending/...)
    calculation_snapshot: any
}

export interface SnapTokenResponse {
    payment_id: string
    order_id: string
    snap_token: string
    redirect_url: string
}
