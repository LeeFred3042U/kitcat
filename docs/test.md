# kitcat Testing Guidelines & Standards

kitcat uses **two distinct test classes** with **non-overlapping rules**:

- **Unit Tests** – pure, in-memory, logic-level verification
- **Integration Tests** – filesystem-backed, repository-level verification

These are not interchangeable.
A test **must clearly belong to exactly one class**

---

## 1. Unit Tests (Logic-Level, Pre-`.git`)

Unit tests validate **pure logic** and **Git semantics**, not repository behavior

### 1.1 What Unit Tests Are Allowed to Do

Unit tests **MAY**:

- operate entirely in memory
- use `[]byte`, `string`, `[]string`, `io.Reader`
- test parsers, algorithms, validators, helpers
- live next to the code they test
- use Go’s standard `testing` package

Unit tests **MUST**:

- be deterministic
- test exactly one logical unit
- assert Git _data semantics_, not CLI output

### 1.2 What Unit Tests Must NEVER Do

Unit tests **MUST NOT**:

- touch the filesystem
- call `t.TempDir`
- call `os.Chdir`
- read or write `.kitcat` or `.git`
- shell out to `git`
- depend on environment variables (`HOME`, `EDITOR`, etc)
- initialize or mutate repositories

If a test does any of the above, it is **not a unit test**

### 1.3 Unit Test Placement & Style

- Tests live **next to the code**
- Same package name (white-box testing)

Example:

```text
internal/diff/myers.go
internal/diff/myers_test.go   // package diff
```

- Use table-driven tests where behavior varies
- Use only `testing`, `t.Fatalf`, `t.Errorf`
- No external assertion libraries

---

## 2. Integration Tests (Repository & Filesystem)

Integration tests verify **observable behavior** of KitCat commands operating on a real repository layout

These tests **intentionally** touch disk and process state

All existing tests fall into this category

### 2.1 Environment Isolation Policy (Integration Tests Only)

Integration tests **MUST** isolate their environment

- **Sandbox Creation:** Use `t.TempDir()` to create a test workspace
- **Process Movement:** Use `os.Chdir()` so relative paths resolve correctly
- **State Restoration:** Always restore CWD and environment variables using `defer`
- **Environment Mocking:** Mock global state (`$HOME`, `$EDITOR`) when required

```go
tempDir := t.TempDir()
origDir, _ := os.Getwd()
origHome := os.Getenv("HOME")

os.Setenv("HOME", tempDir)
os.Chdir(tempDir)

defer func() {
    os.Chdir(origDir)
    os.Setenv("HOME", origHome)
}()
```

---

## 3. Integration Test Coverage Model (“Three-State” Rule)

Every **integration-tested command** must cover the following states

### A. Uninitialized (Zero) State

Verify behavior before `kitcat init`

- Command must fail gracefully
- Must return a clear “not a kitcat repository” error
- Explicitly assert `err != nil`

---

### B. Happy Path (Success)

Verify correct behavior under ideal conditions

- Do **not** stop at `err == nil`
- Physically inspect filesystem side effects
- Verify correct files are created or modified

---

### C. Edge Cases & Errors

Commands must be tested against hostile or awkward inputs:

- **Path traversal** (`../root`)
- **Duplicates** (tags, branches, commits without `--force`)
- **Empty inputs** (empty messages, paths, hashes)
- **Missing files** (files deleted before `add` / `rm`)
- **Empty state** (list commands return empty slices, not errors)

---

## 4. Atomic & Physical Verification (Integration Only)

When a command mutates repository state:

1. Execute the command
2. Inspect the filesystem manually:
   - `os.Stat`
   - `os.ReadFile`

3. Verify:
   - file existence
   - content accuracy
   - data format correctness (e.g. valid JSON)

Example:

- tag file contains the correct commit hash
- index or commits file is well-formed

---

## 5. Integration vs Unit Test Classification (Mandatory)

| Behavior                   | Unit Test | Integration Test       |
| -------------------------- | --------- | ---------------------- |
| Uses `t.TempDir`           | ❌        | ✅                     |
| Uses `os.Chdir`            | ❌        | ✅                     |
| Touches `.kitcat`          | ❌        | ✅                     |
| Touches `.git`             | ❌        | 🚫 (not yet supported) |
| Uses only memory           | ✅        | ❌                     |
| Tests parsing / algorithms | ✅        | ❌                     |
| Tests commands / workflows | ❌        | ✅                     |

If a test violates unit-test rules, it **must be classified as integration**.

---

## 6. Current Scope Constraints (Important)

As of now:

- `.git` scanning is **not implemented**
- `internal/storage` is **not unit-testable**
- No new storage refactors are in scope
- All existing tests are **integration tests by definition**

Do **not** write tests that pretend otherwise.

---

## 7. External Process Handling (Integration Only)

Commands that invoke external tools (e.g. `rebase -i`) must:

- mock required environment variables
- avoid blocking user interaction
- restore all state with `defer`

Example:

```go
os.Setenv("EDITOR", "true")
defer os.Unsetenv("EDITOR")
```

---

## One-line Rule (Do Not Break This)

> If a test touches disk or process state, it is an integration test
> If it touches only memory, it may be a unit test
