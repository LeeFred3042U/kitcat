# Security Policy

As a Version Control System (VCS), the integrity of repository data and the safety of the underlying filesystem are our **core operational requirements**. We value the work of security researchers in identifying vulnerabilities that could lead to data loss or corruption.

> [!IMPORTANT]
> **Do NOT open a GitHub issue or pull request for security-related problems.** Publicly disclosing a vulnerability could allow malicious actors to exploit it before a patch is available.

## Supported Versions

| Version | Supported | Architecture |
| :--- | :--- | :--- |
| Latest commit on `main` (HEAD) | Supported | amd64, arm64 |
| All tagged releases | Unsupported | N/A |
| All forks/patch branches | Unsupported | N/A |

> [!NOTE]
> Reports may be closed if the vulnerability is not reproducible on the latest `main` commit.

## Threat Model
kitcat assumes the working directory may contain malicious files. Commands must never execute untrusted binaries, follow unsafe links, or write outside the repository root.

## Scope
Treat the following as **security issues**, not normal bugs:

* **Arbitrary Code Execution**: Arbitrary Code Execution via binary invocation, hooks, or user-controlled inputs.
* **Path Traversal**: Commands that could write/read files outside the repository root boundary.
* **Data Corruption**: Non-deterministic object/index divergence or checksum mismatch.
* **Unsafe Filesystem Operations**: Checkout/reset behavior that destroys user data without intent.
* **Link Abuse**: Symlink or hardlink abuse that escapes repository boundaries.
* **Malicious Content**: Malicious repository contents causing unintended filesystem writes.

## Out-of-Scope
The following are considered out-of-scope for security reports:
* Third-party Go dependencies.
* User shell configuration.
* External diff/merge tools.

---

## Vulnerability Report Template
Please use the following template when submitting a report to ensure a prompt evaluation.

**Summary:** [A brief, one-sentence description of the vulnerability]

**Reproduction Steps:** [List the exact, minimal commands required to trigger the issue]
1. `kitcat init`
2. ...

**Filesystem Type:** [e.g., ext4, NTFS, APFS, Btrfs — Critical for tracking data corruption bugs]

**Case Sensitivity:** [Is the underlying filesystem case-sensitive? (Yes/No)]

**Environment:** - OS: [e.g., Ubuntu 22.04, macOS 14]
- Go Version: [e.g., go1.24.0]
- kitcat Commit Hash: [The output of `git rev-parse HEAD` in the kitcat repo]
The maintainer aims to acknowledge reports promptly on a **best-effort basis**. Please email your report to: **zeeshanalavi1@gmail.com**.

## Disclosure & Researcher Recognition

1. We will assess the report internally and develop fixes privately.
2. Once a patch is merged, researchers will be **credited in the Release Notes** and acknowledged in the **Commit Messages**.
3. Public disclosure will occur only after a fix is available on `main`.

---
> [!CAUTION]
> **Disclaimer:** kitcat is an educational project and may not be suitable for production-critical data.
