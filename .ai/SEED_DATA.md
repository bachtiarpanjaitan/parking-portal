# Seed Data

> **Convention:** UUIDs are illustrative; production seeders should generate
> them at insert time. Timestamps are fixed relative to "now" so the demo
> repeat-multiplier logic is testable. The 90-day window is anchored to the
> newest seeded violation (`V-NEW`).

> ⚠️ **All amounts in IDR.**

---

# Users

Officer
```json
{
  "id": "11111111-1111-1111-1111-111111111111",
  "name": "Officer User",
  "email": "officer@example.com",
  "role": "OFFICER"
}
```

Member (primary)
```json
{
  "id": "22222222-2222-2222-2222-222222222222",
  "name": "Member User",
  "email": "member@example.com",
  "role": "MEMBER"
}
```

Member (second, for variety in officer screens)
```json
{
  "id": "33333333-3333-3333-3333-333333333333",
  "name": "Member Two",
  "email": "member2@example.com",
  "role": "MEMBER"
}
```

---

# Fine Rule Version 1 (initial, active)

```json
{
  "id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "version_number": 1,
  "is_active": true,
  "published_at": "2026-01-01T00:00:00Z",
  "created_by": "11111111-1111-1111-1111-111111111111"
}
```

## Rule Details (4 rows)

`violation_type` → `base_amount` (all share same multipliers; defaults listed)

| violation_type     | base_amount | day | night | repeat_0 | repeat_1 | repeat_2_plus |
| ------------------ | ----------- | --- | ----- | -------- | -------- | ------------- |
| expired_meter      | 50000       | 1.0 | 1.5   | 1.0      | 1.5      | 2.0           |
| no_parking_zone    | 150000      | 1.0 | 1.5   | 1.0      | 1.5      | 2.0           |
| blocking_hydrant   | 250000      | 1.0 | 1.5   | 1.0      | 1.5      | 2.0           |
| disabled_spot      | 500000      | 1.0 | 1.5   | 1.0      | 1.5      | 2.0           |

---

# Demo Violations (to make History page interesting)

> Anchor time: `T0 = 2026-06-10T10:00:00Z` (newest demo violation)
> All `violation_timestamp` are in UTC.

## V-OLD-1 (paid) — created ~80 days before T0

```json
{
  "id": "v0000001-0000-0000-0000-000000000001",
  "member_id": "22222222-2222-2222-2222-222222222222",
  "rule_version_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "license_plate": "B1234XYZ",
  "violation_type": "no_parking_zone",
  "location": "Jl. Sudirman, Jakarta",
  "violation_timestamp": "2026-03-20T10:00:00Z",
  "photo_url": "/uploads/violations/seed-old-1.jpg",
  "fine_amount": 150000,
  "calculation_snapshot": {
    "rule_version_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
    "rule_version_number": 1,
    "violation_type": "no_parking_zone",
    "base_amount": 150000,
    "time_multiplier": 1.0,
    "time_window": "DAY",
    "repeat_multiplier": 1.0,
    "prior_unpaid_count": 0,
    "calculated_fine": 150000,
    "calculated_at": "2026-03-20T10:00:00Z"
  }
}
```
Invoice `i0000001-...` — status `PAID`
Payment `p0000001-...` — status `PAID`, scenario `success`, transaction `trx_seed_001`

## V-OLD-2 (pending) — created ~40 days before T0, repeat offender

```json
{
  "id": "v0000002-0000-0000-0000-000000000002",
  "member_id": "22222222-2222-2222-2222-222222222222",
  "rule_version_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "license_plate": "B1234XYZ",
  "violation_type": "blocking_hydrant",
  "location": "Jl. Thamrin, Jakarta",
  "violation_timestamp": "2026-05-01T23:30:00Z",
  "photo_url": "/uploads/violations/seed-old-2.jpg",
  "fine_amount": 750000,
  "calculation_snapshot": {
    "rule_version_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
    "rule_version_number": 1,
    "violation_type": "blocking_hydrant",
    "base_amount": 250000,
    "time_multiplier": 1.5,
    "time_window": "NIGHT",
    "repeat_multiplier": 2.0,
    "prior_unpaid_count": 1,
    "calculated_fine": 750000,
    "calculated_at": "2026-05-01T23:30:00Z"
  }
}
```
> Calculation: 250000 × 1.5 × 2.0 = 750000. The "1 prior unpaid" comes from V-OLD-1
> which, at the time V-OLD-2 was created, had not yet been paid.

Invoice `i0000002-...` — status `PENDING`
(no payment yet)

## V-OLD-3 (failed payment) — created ~10 days before T0

```json
{
  "id": "v0000003-0000-0000-0000-000000000003",
  "member_id": "22222222-2222-2222-2222-222222222222",
  "rule_version_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "license_plate": "B1234XYZ",
  "violation_type": "disabled_spot",
  "location": "Jl. Gatot Subroto, Jakarta",
  "violation_timestamp": "2026-05-30T08:00:00Z",
  "photo_url": "/uploads/violations/seed-old-3.jpg",
  "fine_amount": 500000,
  "calculation_snapshot": {
    "rule_version_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
    "rule_version_number": 1,
    "violation_type": "disabled_spot",
    "base_amount": 500000,
    "time_multiplier": 1.0,
    "time_window": "DAY",
    "repeat_multiplier": 1.0,
    "prior_unpaid_count": 0,
    "calculated_fine": 500000,
    "calculated_at": "2026-05-30T08:00:00Z"
  }
}
```
Invoice `i0000003-...` — status `FAILED`
Payment `p0000003-...` — status `FAILED`, scenario `failed`, transaction `trx_seed_003`

> Note: the V-OLD-3 invoice is `FAILED` and **still payable** (member can retry).

## V-NEW (pending, ready for member to pay) — created at T0

```json
{
  "id": "v0000004-0000-0000-0000-000000000004",
  "member_id": "22222222-2222-2222-2222-222222222222",
  "rule_version_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "license_plate": "B1234XYZ",
  "violation_type": "expired_meter",
  "location": "Jl. Kuningan, Jakarta",
  "violation_timestamp": "2026-06-10T10:00:00Z",
  "photo_url": "/uploads/violations/seed-new.jpg",
  "fine_amount": 75000,
  "calculation_snapshot": {
    "rule_version_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
    "rule_version_number": 1,
    "violation_type": "expired_meter",
    "base_amount": 50000,
    "time_multiplier": 1.0,
    "time_window": "DAY",
    "repeat_multiplier": 1.5,
    "prior_unpaid_count": 1,
    "calculated_fine": 75000,
    "calculated_at": "2026-06-10T10:00:00Z"
  }
}
```
> Calculation: 50000 × 1.0 × 1.5 = 75000. The "1 prior unpaid" is V-OLD-3
> (which is `FAILED`, hence still unpaid in the 90-day window).

Invoice `i0000004-...` — status `PENDING`
(no payment yet — member will exercise the success/failed scenario here)

---

# Expected repeat-multiplier behavior at T0 (2026-06-10)

| Plate       | Open invoices in last 90d (PENDING|FAILED) | repeat |
| ----------- | ------------------------------------------ | ------ |
| B1234XYZ    | V-OLD-2 (PENDING) + V-OLD-3 (FAILED) = 2   | 2.0    |
| (other)     | 0                                          | 1.0    |

So if a NEW violation is created for `B1234XYZ` at T0, the repeat multiplier
will be **2.0** (2 prior unpaid). The first violation V-NEW was created with
only 1 prior unpaid because V-OLD-2 was still PENDING at the time V-NEW was
seeded; in a fresh DB where seeding is replayed in order, the snapshot already
captures the count at the time of that violation.

---

# Notes for the seeder script

- The seeder should run in this order:
  1. `INSERT users` (3 rows)
  2. `INSERT fine_rule_versions` (V1 active)
  3. `INSERT fine_rule_details` (4 rows for V1)
  4. `INSERT violations` (V-OLD-1, V-OLD-2, V-OLD-3, V-NEW) **in order**
  5. `INSERT invoices` (one per violation, in same order)
  6. `INSERT payments` (V-OLD-1 paid, V-OLD-3 failed)
- The 4 photo files referenced by `photo_url` should be placed in
  `storage/violations/` before the first run. They can be any small valid JPG/PNG.
- After seeding, run `GET /history?member_id=22222222-...` to confirm all
  4 rows show with the correct status, fine, and rule_version_number.
