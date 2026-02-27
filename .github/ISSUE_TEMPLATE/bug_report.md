---
name: Bug Report
about: Report a concrete bug with clear scope and fix boundaries
title: "[BUG] "
labels: bug
assignees: ""
---

## Description

Clearly describe **what is broken** and **why it is wrong**.
Assume the reader has repo context but has not seen this bug before.

---

## Impact / Risk

Explain the consequence if this bug is left unfixed.
Examples:
- Silent data loss
- Incorrect CLI behavior
- Inconsistent state
- UX confusion

Be explicit. No hand-waving.

---

## Reproduction Steps

Steps to reproduce the behavior **exactly**:

1. Run command `kitcat ...`
2. Perform action `...`
3. Observe error / incorrect behavior

If it is not reproducible, it is not actionable.

---

## Expected Behavior

Describe the **correct** behavior according to intended design or Git-like semantics.

---

## Actual Behavior

Describe what actually happens.
Include error messages, exit codes, or silent failures.

---

## Technical Constraints

List **what must NOT be changed**.
Examples:
- Do not modify `core.RemoveFile`
- Do not touch `storage` package
- Do not alter flag behavior

This section is mandatory if constraints exist.

---

## Required Code Changes

**File(s):**
- `path/to/file.go`

**Function(s):**
- `FunctionName`

High-level description of where and how the fix should occur.
No vague “refactor as needed”.

---

## Logic Requirements

Bullet-point the required logic or ordering guarantees.
Example:
- Must fail before any files are written
- Must attempt all operations before exiting
- Must return non-zero on partial failure

This is where ambiguity goes to die.

---

## Forbidden Changes

Explicitly list disallowed approaches.
Example:
- Changing core APIs
- Suppressing errors
- Adding global flags

---

## Acceptance Criteria

Concrete conditions that must be true for this issue to be considered resolved.
Example:
- `kitcat rm file1 file2` removes both files
- `kitcat rm` exits with code 2 and prints usage
- No files are modified on failure

If it can’t be verified, it doesn’t belong here.

---

## Possible Fix (Optional)

Optional implementation hints.
This is guidance, not a free-for-all.
