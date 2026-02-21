# kitcat

[![SWOC Season 6](https://img.shields.io/badge/SWOC-Season%206-blue?style=for-the-badge&logo=codeforces)](https://swoc.tech)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**kitcat** is a lightweight, learning-focused Git clone written in Go. It is designed to help developers understand the internal mechanics of version control by implementing core Git logic from scratch.

---

## Quick Start (TL;DR)

Get `kitcat` up and running on your local machine:

### 1. Installation

```bash
# Clone and build
git clone https://github.com/LeeFred3042U/kitcat.git
cd kitcat
go build -o kitcat ./cmd/main.go
```

### 2. First-Time Configuration

```
./kitcat config --global user.name "Your Name"
./kitcat config --global user.email "you@example.com"
```

### 3. Basic Workflow

```
./kitcat init
./kitcat add <file>
./kitcat commit -m "Initial commit"
```

---

## How it Works (Internal Design)

kitcat mimics the core principles of Git, operating on **snapshots** built from three key objects: **Blobs** (content), **Trees** (structure), and **Commits** (history).

Instead of high-level abstractions, we encourage you to explore the internal architecture and command logic in our dedicated documentation:
**[Read the Architecture Guide](./docs/ARCHITECTURE.md)**

---

## What’s Supported vs. What’s Not

kitcat implements a functional subset of Git's "Plumbing" and "Porcelain" commands.

> [!IMPORTANT]
> **A Note on Flags:** kitcat implements a **strict subset of Git flags**. For example, we support `commit -m` but **not** flags like `--author`, `--date`, or others. This restricted flag support applies to all commands across the project.

| Feature            | Supported                                       | Not Supported                           |
| :----------------- | :---------------------------------------------- | :-------------------------------------- |
| **Local Workflow** | Init, Add, Commit, Status                       | Staging specific hunks, Interactive add |
| **History**        | Log, Branching, Checkout, Rebase (Experimental) | Cherry-pick, Reflog                     |
| **Merging**        | Fast-Forward (FF) Only                          | Merge conflict resolution, 3-way merges |
| **Collaboration**  | Local directory only                            | Remotes (Push, Pull, Fetch, Remote)     |

---

## Command Reference Summary

| Command    | Action                               | Usage Example                  |
| :--------- | :----------------------------------- | :----------------------------- |
| `init`     | Create a new `.kitcat` repository.   | `./kitcat init`                |
| `add`      | Stage files to the index.            | `./kitcat add --all`           |
| `commit`   | Record changes to the repository.    | `./kitcat commit -m "msg"`     |
| `status`   | Show working directory state.        | `./kitcat status`              |
| `diff`     | View colorized diff (Index vs HEAD). | `./kitcat diff`                |
| `log`      | View commit history.                 | `./kitcat log --oneline`       |
| `branch`   | List or create branches.             | `./kitcat branch feature`      |
| `checkout` | Switch branches or restore files.    | `./kitcat checkout main`       |
| `merge`    | Join histories (**FF-only**).        | `./kitcat merge feature`       |
| `clean`    | Remove untracked files.              | `./kitcat clean -f`            |
| `config`   | Set user name and email.             | `./kitcat config --global ...` |
| `rebase`   | Reapply commits on another branch.   | `./kitcat rebase -i HEAD~3`    |
| `stash`    | Stash changes in working directory.  | `./kitcat stash`               |
| `shortlog` | Summarize commit history.            | `./kitcat shortlog`            |
| `grep`     | Print lines matching a pattern.      | `./kitcat grep "TODO"`         |
| `rm`       | Remove files from working tree.      | `./kitcat rm file.txt`         |
| `mv`       | Move or rename a file.               | `./kitcat mv old new`          |
| `tag`      | Create a tag for a commit.           | `./kitcat tag v1.0 abc1234`    |
| `reset`    | Reset current HEAD to state.         | `./kitcat reset --hard abc123` |

---

## Key Features & Usage

### Ignoring Files (`.kitignore`)

Create a `.kitignore` file in the root to exclude patterns:

- **Glob patterns:** `*.log`, `file?.dat`
- **Directories:** `bin/`, `node_modules/`
- **Recursive:** `**/*.tmp`, `**/.cache`

### Getting Help

You can get detailed information for any command directly from the CLI:

```bash
./kitcat help
./kitcat help add
./kitcat help commit
./kitcat log
```

---

## Contributing

We welcome contributors who want to learn! Whether you're fixing a bug or improving docs, your help is appreciated.
Please read our **[CONTRIBUTING.md](./CONTRIBUTING.md)** for developer setup, coding standards, and contribution guidelines before submitting a Pull Request.

---

## Reference Material

Refer to the official Git documentation, to understand how git commands work:

- [Git Documentation (Official)](https://git-scm.com/docs/git)

---
> [!CAUTION]
> **Disclaimer:** kitcat is an educational project and may not be suitable for production-critical data.
