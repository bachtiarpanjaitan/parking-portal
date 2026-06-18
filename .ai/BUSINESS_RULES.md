# Business Rules

> ⚠️ **Time window decision:** The assignment PDF writes the day/night window
> as `06:00 – 22:00` and `22:00 – 06:00`, which creates a boundary overlap at
> 22:00. This document **chooses half-open intervals** (the more common
> convention and the one used by the reference test cases in
> `TESTING_STRATEGY.md`):
> - **Day:** 06:00:00 – 21:59:59 (inclusive) → multiplier `1.0`
> - **Night:** 22:00:00 – 05:59:59 (inclusive) → multiplier `1.5`
>
> This is also the convention coded in the service layer's time-window helper.
> Any ticket touching this rule should reference this paragraph.

---

## Rule Versioning

The system must support multiple fine rule versions.

Only one rule version can be active at a time.

A violation must always use the currently active rule version.

Past violations must never be recalculated.

Every violation stores:
- `rule_version_id` (FK)
- `calculation_snapshot` (JSONB)

---

## Violation Creation

Officer can create violations.

Required fields:
- `license_plate`
- `violation_type`
- `location`
- `violation_timestamp` (UTC) — when the violation occurred
- `photo_url` — from `POST /uploads/violations` (see `PHOTO_STORAGE.md`)
- `member_id` — looked up from `users` (plate is not unique to a member, so
  officer must confirm which member is associated with the plate)

Creating a violation automatically:
1. Loads the **currently active** rule version
2. Loads/calculates the **repeat multiplier** by counting prior unpaid violations
   for the same `license_plate` in the last 90 days (based on `violation_timestamp`)
3. Calculates the fine
4. Persists the violation with the frozen `rule_version_id` and `calculation_snapshot`
5. Creates an `invoice` row in `PENDING` status

> Officers **cannot** pick a rule version. The system always uses the active one.

---

## Fine Calculation

Formula:

```
fine = base_amount × time_multiplier × repeat_multiplier
```

All three factors come from the active `FineRuleDetail` for the violation's type.

---

## Time Multiplier

| Window            | Local time    | Multiplier |
| ----------------- | ------------- | ---------- |
| Day               | 06:00 – 21:59 | 1.0        |
| Night             | 22:00 – 05:59 | 1.5        |

- The window is evaluated against the **local time** of `violation_timestamp`.
  (Server may run in UTC; the conversion to local time is the officer's
  jurisdiction — see "Assumptions" in the README.)

---

## Repeat Multiplier

Count **unpaid** violations (invoices with status `PENDING` or `FAILED`) for the
**same `license_plate`** whose `violation_timestamp` falls within the **last 90
days** of the new violation's `violation_timestamp`.

| Prior unpaid (last 90d) | Multiplier |
| ----------------------- | ---------- |
| 0                       | 1.0        |
| 1                       | 1.5        |
| 2 or more               | 2.0        |

The 3 values map to the `fine_rule_details` columns `repeat_0`, `repeat_1`,
`repeat_2_plus` respectively (see `DATABASE_MAPPING.md`).

---

## Invoice Rules

- One violation creates exactly one invoice.
- Invoice `amount` equals `violation.fine_amount` at creation time.
- Invoice `amount` never changes.
- Initial invoice status is `PENDING`.

---

## Payment Rules

- Only invoices in `PENDING` or `FAILED` status can be paid.
- `PAID` invoices cannot be re-paid.
- Payment result comes from the mocked `PaymentService.charge()`.
- Possible outcomes:
  - `success` → invoice becomes `PAID`, payment row is `PAID`
  - `failed` → invoice becomes `FAILED`, payment row is `FAILED`,
    member can retry (status `FAILED` is payable)

> The mock provider does not actually validate the amount. The service layer
> enforces that `payments.amount == invoices.amount` for audit purposes.

---

## History Rules

History must display for each violation:
- violation id, type, location, timestamp
- `fine_amount`
- `rule_version_number` (and id)
- invoice status
- most recent payment status
- the full `calculation_snapshot` (for transparency)

Historical records are **immutable**:
- Violations cannot be edited or deleted.
- Invoices cannot be edited; only status transitions are allowed.
- Payments cannot be edited; new attempts create new rows.
- Rule versions cannot be edited after publish.
