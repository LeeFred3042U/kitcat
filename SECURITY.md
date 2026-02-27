# Security Policy

## Reporting Security Issues

**Do NOT open a GitHub issue or pull request for security-related problems.**

All security bugs, vulnerabilities, data corruption issues, or unsafe behaviors
**must be reported privately by email**.

📧 **Send reports to:**  
**zeeshanalavi1@gmail.com**

This includes (but is not limited to):

- Repository corruption
- Data loss or silent failure
- Index, object, or ref inconsistencies
- Unsafe filesystem operations
- Checkout/reset behavior that overwrites user data
- Crashes that leave repos in an unrecoverable state
- Any behavior that could break `.git` compatibility in the future

---

## What Counts as a Security Issue

Treat the following as **security issues**, not normal bugs:

- Silent corruption of `.kitcat` repositories
- Incorrect handling of user data on disk
- Overwriting files without explicit user intent
- Unsafe defaults that can destroy local changes
- Index/object mismatches that brick repositories
- Crashes during write operations that leave partial state
- Any bug that could cause permanent data loss

If in doubt, **report it as a security issue**.

---

## How to Report (Required Format)

Your email **must** include the following:

### 1. Summary

One or two sentences describing the issue.

### 2. Affected Area

Specify exactly what is affected:

- Command(s)
- Files or directories
- Storage / index / object layer
- Branch (main / develop)

### 3. Reproduction Steps

Exact, minimal steps to reproduce the issue.
Include commands run and files touched.

### 4. Impact

Explain what can go wrong:

- Data loss
- Repo corruption
- Incorrect behavior
- Crash / denial of service

### 5. Environment

- OS
- KitCat version / commit hash
- Go version

Reports missing this information may be ignored.

---

## What NOT to Do

- ❌ Do NOT open a public GitHub issue for security problems
- ❌ Do NOT submit a pull request attempting to “fix” a security issue
- ❌ Do NOT discuss security issues publicly before maintainers respond
- ❌ Do NOT attach large archives or binaries without asking first

Security fixes require coordination and may involve design decisions.
Unreviewed PRs touching sensitive areas will be closed.

---

## Disclosure & Fix Process

- We will acknowledge receipt of your report.
- We will assess severity and impact internally.
- Fixes will be developed privately if needed.
- Public disclosure (if any) will happen **after** a fix is available.

There is **no guaranteed response time**, but high-impact issues are prioritized.

---

## Relationship to Contributions

Security issues are **not** normal contributions.

- Normal bugs → GitHub issues + PRs (see `CONTRIBUTING.md`)
- Security issues → **private email only**
- PRs touching security-sensitive areas without prior coordination will be rejected

This policy exists to protect users from data loss and maintain repository integrity

---

## Final Note

If your report involves `.git`, `.kitcat`, object storage, index files, or checkout/reset behavior, assume it is security-sensitive and report it privately

When in doubt: **email first**

