---
name: Test
about: Add or improve test coverage (unit, integration, or hybrid)
title: "[TEST] "
labels: test
assignees: ""
---

## Test Type

Select exactly one:

- [ ] Unit — isolated logic, no filesystem, no cross-package behavior
- [ ] Integration — multiple components, real filesystem/state
- [ ] Hybrid — realistic workflow with partial mocking

If you check more than one, the issue is invalid.

---

## Area Under Test

- Package(s):
- File(s):
- Function(s) / Command(s):

Be concrete. Vague = rejected.

---

## Scenario / Behavior

Describe the behavior or workflow being tested.
This should read like a deterministic script, not a story.

---

## Why This Test Is Needed

What risk does this test prevent?
Examples:
- Regression
- Data loss
- Silent failure
- Incorrect CLI behavior

If there is no clear risk, this test is unnecessary.

---

## Real vs Mocked Components

**Real:**
- 

**Mocked / Stubbed:**
- 

For unit tests, “Real” should be basically empty.

---

## Constraints

List any hard constraints:
- No production logic changes
- No external dependencies
- Deterministic execution only

---

## Acceptance Criteria

- Test fails before the fix
- Test passes after the fix
- Test is stable and repeatable
