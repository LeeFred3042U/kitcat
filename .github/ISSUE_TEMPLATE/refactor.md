---
name: Refactor / Tech Debt
about: Improve internal structure without changing behavior
title: "[REFACTOR] "
labels: refactor
assignees: ""
---

## Current Problem

Describe what is wrong with the current code:
- Duplication
- Inconsistent logic
- Poor separation of concerns

Be specific. Cite files and functions.

---

## Why This Matters

Explain the cost of leaving this as-is:
- Bugs
- Inconsistent behavior
- Contributor confusion
- Maintenance overhead

---

## Scope of Change

List **exactly** what is allowed to change.
Examples:
- Command parsing only
- Internal helpers only

If behavior must remain identical, state that explicitly.

---

## Explicit Non-Goals

What this refactor must NOT do:
- No new features
- No behavior changes
- No API changes

If behavior *will* change, this is the wrong template.

---

## Technical Constraints

Hard rules.
Examples:
- Do not modify core APIs
- Do not introduce new dependencies

---

## Acceptance Criteria

- Behavior before and after is identical
- Tests (if any) still pass
- Code paths are consolidated or simplified
