# Security Policy

As a Version Control System (VCS), the integrity of repository data and the safety of the underlying filesystem are our highest priorities. We value the work of security researchers in identifying vulnerabilities that could lead to data loss or corruption.

> [!IMPORTANT]
> **Do NOT open a GitHub issue or pull request for security-related problems.** Publicly disclosing a vulnerability could allow malicious actors to exploit it before a patch is available.

## Supported Versions

| Version | Supported |
| :--- | :--- |
| Latest commit on `main` | Yes |
| All other branches/tags | No |

Security fixes are currently provided **only for the latest commit on the `main` branch**. We recommend all users stay up to date to ensure maximum data protection.

## What Counts as a Security Issue?

Treat the following as **security issues**, not normal bugs:
* **Arbitrary Code Execution**: Vulnerabilities in the Go binary execution.
* **Path Traversal**: Commands that could write/read files outside the intended `.kitcat` scope.
* **Data Corruption**: Silent corruption of `.kitcat` repositories or index/object mismatches.
* **Unsafe Filesystem Operations**: Checkout/reset behavior that destroys user data without intent.

## How to Report (Required Format)

The maintainer aims to acknowledge reports promptly on a **best-effort basis**. Please email your report to: **zeeshanalavi1@gmail.com**.

To help us investigate, please include the following **VCS-specific details**:

1. **Summary**: A brief description of the vulnerability.
2. **Reproduction Steps**: Exact, minimal commands to trigger the issue.
3. **Filesystem Type**: (e.g., ext4, NTFS, APFS, Btrfs) — *Critical for tracking data corruption bugs.*
4. **Case Sensitivity**: Is the underlying filesystem case-sensitive?
5. **Environment**: OS, Go version, and KitCat commit hash.

## Disclosure & Researcher Recognition

1. We will assess the report internally and develop fixes privately.
2. Once a patch is merged, researchers will be **credited in the Release Notes** and acknowledged in the **Commit Messages**.
3. Public disclosure will occur only after a fix is available on `main`.

---
> [!CAUTION]
> **Disclaimer:** kitcat is an educational "toy" project. While we strive for security, do not use it as your primary version control for production-critical data.
