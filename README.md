# kitcat

A Git-compatible version control system built from scratch in Go. kitcat implements Git's core object model — content-addressed storage, a binary index, zlib-compressed objects, and a full porcelain command set — using the same on-disk format as Git itself.

```
$ kitcat init
$ kitcat add .
$ kitcat commit -m "initial commit"
$ kitcat log --oneline
```

---

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Architecture](#architecture)
- [Object Model](#object-model)
- [Index (Staging Area)](#index-staging-area)
- [Command Reference](#command-reference)
- [Diff Engine](#diff-engine)
- [Merge Engine](#merge-engine)
- [Rebase Sequencer](#rebase-sequencer)
- [Configuration](#configuration)
- [Ignore Rules](#ignore-rules)
- [CI Automation](#ci-automation)
- [Contributing](#contributing)

---

## Overview

kitcat is a ground-up reimplementation of Git's fundamental mechanics. It stores objects in the same SHA-1 content-addressed format, uses the same zlib compression, reads and writes the same INI-format config files, and produces the same commit object layout — down to the `tree`, `parent`, `author`, and `committer` header lines.

The goal is correctness over compatibility at the margins. A kitcat repository can be renamed from `.kitcat` to `.git` and inspected with standard Git tooling, because the underlying formats match.

**What kitcat implements:**

- Full porcelain: `init`, `add`, `commit`, `status`, `log`, `diff`, `branch`, `checkout`, `merge`, `rebase`, `reset`, `stash`, `tag`, `mv`, `rm`, `ls-files`, `clean`, `grep`, `config`, `show-object`
- Git-compliant binary index with stat caching, file mode tracking, and exclusive write locking
- Content-addressed object database (blob, tree, commit, tag) with fan-out directory structure
- Myers diff algorithm with string interning for performance
- Three-way text merge engine with conflict marker generation
- Interactive rebase sequencer with conflict pause/resume
- Annotated and lightweight tags
- Reflog for HEAD and branch references
- `.kitignore` pattern matching with glob and `**` support
- Atomic file writes with Windows-safe retry logic
- Git-compliant INI config with local and global scope

---

## Installation

**Requirements:** Go 1.24+

```bash
git clone https://github.com/LeeFred3042U/kitcat
cd kitcat
go build -o kitcat ./cmd/main.go
```

Move the binary somewhere on your `$PATH`:

```bash
mv kitcat /usr/local/bin/kitcat
```

Or run it directly from the repo root after building.

---

## Architecture

kitcat is organized into distinct layers, each with a single responsibility:

```
cmd/                     CLI entry point — argument dispatch
internal/
  app/                   App-level constants (name)
  repo/                  Path constants (.kitcat/*, index, HEAD)
  models/                Commit struct
  plumbing/              Low-level object writers
    hash_object.go       SHA-1 hashing, zlib compression, object storage
    write_tree.go        Recursive tree object construction
    write_commit.go      Commit object serialization
    read_index.go        Binary index deserialization
    index_entry.go       IndexEntry type definition
    update_index.go      Index serialization
  storage/               Object database adapter
    adaptor.go           ReadObject, FindCommit, FindMergeBase, ReadCommits
    index.go             LoadIndex, UpdateIndex, WriteIndexFromTree
    tree.go              ParseTree (binary + legacy format), CreateTree
    stash_store.go       Stash stack push/pop/list
    safe_io.go           File I/O helpers
    lockUnix.go          Unix file locking
    lockOther.go         Windows file locking (spin-lock)
  diff/
    myers.go             Generic Myers diff algorithm
  merge/
    engine.go            MergeTrees — pure 3-way tree diff
    merge3.go            Merge3 — line-level 3-way text merge
    apply.go             ApplyMergePlan — disk writes and index update
  core/                  Porcelain commands
    add.go               AddFile, AddAll, stageFile
    branch.go            CreateBranch, DeleteBranch, ListBranches, RenameBranch
    checkout.go          Checkout, CheckoutFile
    clean.go             Clean
    commit.go            Commit, CommitAll, AmendCommit
    config.go            GetConfig, SetConfig, PrintAllConfig
    diff.go              Diff (staged and unstaged)
    grep.go              Grep
    helpers.go           SafeWrite, FindRepoRoot, ReflogAppend, ...
    ignore.go            LoadIgnorePatterns, ShouldIgnore
    index.go             Legacy JSON index (kept for compatibility)
    init.go              Init
    log.go               Log
    ls.go                ListFiles
    merge.go             Merge, MergeAbort
    move.go              MoveFile
    rebase.go            Rebase, RebaseContinue, RebaseAbort
    rebase_state.go      Sequencer state persistence
    remove.go            RemoveFile
    reset.go             Reset, CheckoutTree, ReadTree
    show.go              ShowObject
    stash.go             Stash, StashPush, StashPop, StashApply, StashDrop, StashList, StashClear
    status.go            Status, detectRenames
    tag.go               CreateTag, CreateAnnotatedTag, PrintTags
  testutil/
    setup.go             Test helper utilities
```

**Data flow for a commit:**

```
kitcat add file.go
  → stageFile() hashes content → plumbing.HashAndWriteObject() → .kitcat/objects/xx/...
  → storage.UpdateIndex() acquires lock → writes binary .kitcat/index

kitcat commit -m "msg"
  → plumbing.WriteTree() reads index → recursively builds tree objects
  → plumbing.CommitTree() writes commit object
  → updateHead() writes branch ref → ReflogAppend()
```

---

## Object Model

kitcat stores all repository data as immutable, content-addressed objects in `.kitcat/objects/`. Each object is stored at a path determined by its SHA-1 hash, using a two-level fan-out:

```
.kitcat/objects/
  3a/           ← first two hex chars
    f4b9...     ← remaining 38 chars
```

Every object is stored as:

```
<type> <size>\0<payload>
```

compressed with zlib. The SHA-1 hash is computed over the full header+payload, ensuring that identical content always produces identical hashes and that objects are de-duplicated automatically.

**Object types:**

| Type     | Description                                                           |
| -------- | --------------------------------------------------------------------- |
| `blob`   | Raw file content                                                      |
| `tree`   | Directory snapshot: `<mode> <name>\0<20-byte hash>` per entry         |
| `commit` | Points to a tree hash; contains author, committer, timestamp, message |
| `tag`    | Annotated tag object: points to a commit hash with tagger identity    |

**Commit object format:**

```
tree <tree-hash>
parent <parent-hash>
author Name <email> <unix-timestamp> <tz-offset>
committer Name <email> <unix-timestamp> <tz-offset>

<commit message>
```

This is identical to Git's commit object format, meaning kitcat commits can be read by `git cat-file` after renaming the repo directory.

---

## Index (Staging Area)

The staging area is stored as a binary file at `.kitcat/index`. Each entry records:

| Field       | Description                                |
| ----------- | ------------------------------------------ |
| `Path`      | Repo-relative file path                    |
| `Hash`      | `[20]byte` raw SHA-1 of blob               |
| `Mode`      | Unix file mode (`0100644`, `0100755`)      |
| `Size`      | File size in bytes (used for stat caching) |
| `MTimeSec`  | Modification time seconds                  |
| `MTimeNSec` | Modification time nanoseconds              |
| `Stage`     | Conflict stage (0 = clean, 2 = ours)       |

**Stat caching:** When staging a file, if the existing index entry's size and mtime match the current file's stat, hashing is skipped entirely. This avoids redundant I/O on large unchanged files.

**Exclusive locking:** All index mutations go through `storage.UpdateIndex()`, which acquires a file lock before reading, passes the in-memory map to a callback, then writes the result back. Concurrent writers are serialized; concurrent readers are not blocked.

---

## Command Reference

### Repository Setup

#### `kitcat init`

Initializes a new repository in the current directory. Creates the `.kitcat/` directory structure:

```
.kitcat/
  HEAD           → ref: refs/heads/main
  config         → INI format
  index          → binary index (empty)
  objects/       → object database
  refs/
    heads/       → branch references
    tags/        → tag references
  hooks/         → sample hook scripts
  info/exclude   → local ignore rules
```

Safe to run on an existing repository — existing files are never overwritten.

---

### Staging

#### `kitcat add <path>`

Stages a file or directory. When a directory is given, files are staged recursively while respecting `.kitignore` rules and skipping `.kitcat/` itself.

```bash
kitcat add file.go
kitcat add src/
kitcat add .
```

#### `kitcat add --all` / `kitcat add -A`

Stages all changes in the repository: new files, modifications, and deletions. Previously tracked files that no longer exist on disk are removed from the index.

---

### Committing

#### `kitcat commit -m "<message>"`

Creates a new commit from the current index state. If the repository is in a merge state (MERGE_HEAD exists), the merge commit is finalized automatically with two parents.

```bash
kitcat commit -m "add login flow"
```

#### `kitcat commit` (no message)

Opens your `$EDITOR` (or `$GIT_EDITOR`, falling back to `vi`) to compose the commit message. Lines starting with `#` are stripped. An empty message aborts the commit.

#### `kitcat commit -am "<message>"`

Stages all tracked changes (`add -A`) before committing in a single step.

#### `kitcat commit --amend`

Replaces the most recent commit with a new commit object. The current index state becomes the new commit's tree. Cannot be used during an active merge.

---

### Inspecting State

#### `kitcat status`

Shows the current branch and three sections of changes:

- **Unmerged paths** — files with active conflict markers
- **Changes to be committed** — staged changes (index vs HEAD)
- **Changes not staged for commit** — unstaged changes (working tree vs index)
- **Untracked files** — files not in the index

Rename detection is performed in the staged section using two phases: exact hash match first, then Jaccard line-similarity scoring (>50% threshold) for approximate matches.

#### `kitcat log`

Displays the full commit history from HEAD, following parent pointers. Each entry shows the commit hash, author, date, and message.

```bash
kitcat log
kitcat log --oneline     # compact single-line format
kitcat log -n 5          # limit to 5 commits
```

#### `kitcat diff`

Shows unstaged changes: working directory vs index.

#### `kitcat diff --staged`

Shows staged changes: index vs HEAD.

Both modes use the Myers diff algorithm and print colored `+`/`-` output per line.

#### `kitcat show-object <hash>`

Prints the raw decompressed payload of any object in the database. Useful for inspecting blobs, trees, and commits directly.

---

### Branching

#### `kitcat branch`

Lists all local branches. The current branch is highlighted with a `*`.

#### `kitcat branch <name>`

Creates a new branch pointing to the current HEAD commit.

#### `kitcat branch -m <new-name>`

Renames the currently checked-out branch. Updates HEAD, the ref file, and writes reflog entries for both the old and new names.

#### `kitcat branch -d <name>`

Deletes a branch. Rejected if the branch is currently checked out.

---

### Switching

#### `kitcat checkout <branch>`

Switches to an existing branch or detached commit hash. Before switching:

1. Aborts if the working directory has uncommitted changes (unless `-f` is passed)
2. Checks for untracked files that would be overwritten by files in the target tree

On success, updates the working directory, rebuilds the index inside a transaction, and moves HEAD last. A reflog entry is written.

```bash
kitcat checkout main
kitcat checkout abc1234    # detached HEAD
kitcat checkout -f main    # force, discard local changes
```

#### `kitcat checkout -b <name>`

Creates a new branch and immediately switches to it.

#### `kitcat checkout -- <file>`

Restores a single file from the last commit into the working directory. Refuses to overwrite a file with local modifications or an untracked file that would be clobbered.

---

### Merging

#### `kitcat merge <branch>`

Merges the specified branch into the current branch.

**Fast-forward merge:** If the current HEAD is an ancestor of the target, the branch pointer is simply advanced and the workspace is updated. No merge commit is created.

**Three-way merge:** If the branches have diverged, kitcat computes the merge base, then runs a three-way merge across all affected files:

- Files unchanged by both branches: kept as-is
- Files changed by only one branch: that version wins automatically
- Files changed identically by both branches: that version wins automatically
- Files changed differently by both branches: conflict markers are written

After a three-way merge, `MERGE_HEAD` and `MERGE_MSG` are written to `.kitcat/`. Run `kitcat commit` to finalize the merge commit, or `kitcat merge --abort` to cancel.

**Conflict markers:**

```
<<<<<<< HEAD
ours version of the line
=======
their version of the line
>>>>>>> MERGE_HEAD
```

#### `kitcat merge --abort`

Cancels an active merge, hard-resets back to the pre-merge HEAD, and removes `MERGE_HEAD` and `MERGE_MSG`.

---

### Rebasing

#### `kitcat rebase <branch>`

Replays commits from the current branch onto the tip of the target branch. For each commit to replay:

1. The commit's parent tree is used as the merge base
2. A three-way merge is computed between that base, the current rebased HEAD, and the commit's tree
3. If clean, a new commit is created preserving the original author and timestamp, with the current user as committer
4. If conflicts occur, the sequencer pauses and writes the conflicting commit hash to `rebase-merge/stopped-sha`

```bash
kitcat rebase main
```

#### `kitcat rebase -i <branch>`

Interactive rebase. Opens the todo list in `$EDITOR`. Supported actions:

| Action       | Description       |
| ------------ | ----------------- |
| `pick` / `p` | Apply the commit  |
| `drop` / `d` | Remove the commit |

Removing a line also drops the commit.

#### `kitcat rebase --continue`

After resolving conflicts with `kitcat add`, continues the sequencer from where it paused.

#### `kitcat rebase --abort`

Cancels the rebase and restores HEAD to its pre-rebase state.

---

### Resetting

#### `kitcat reset --soft <commit>`

Moves the branch pointer to the specified commit. Index and working directory are untouched.

#### `kitcat reset --mixed <commit>` (default)

Moves the branch pointer and rebuilds the index to match the target commit. Working directory is untouched.

#### `kitcat reset --hard <commit>`

Moves the branch pointer, rebuilds the index, and overwrites the working directory to exactly match the target commit. Discards all local changes.

```bash
kitcat reset --hard HEAD
kitcat reset --hard abc1234
kitcat reset --hard main
```

---

### Stashing

#### `kitcat stash`

Saves the current working directory and index state as a WIP commit, then hard-resets to HEAD. The stash is pushed onto a stack stored at `.kitcat/refs/stash`.

```bash
kitcat stash
kitcat stash push -m "work in progress on auth"
```

#### `kitcat stash list`

Shows all stashed states, newest first:

```
stash@{0}: WIP on main: abc1234 last commit message
stash@{1}: WIP on feature: def5678 ...
```

#### `kitcat stash pop`

Applies the most recent stash to the working directory and removes it from the stack. Aborts if the working directory is dirty.

#### `kitcat stash apply [<index>]`

Applies the stash at the given index without removing it from the stack.

#### `kitcat stash drop [<index>]`

Removes a stash entry without applying it.

#### `kitcat stash clear`

Removes all stash entries.

---

### Tagging

#### `kitcat tag`

Lists all tags.

#### `kitcat tag <name> <commit>`

Creates a lightweight tag: a named reference pointing directly to a commit hash.

#### `kitcat tag -a -m "<message>" <name> <commit>`

Creates an annotated tag: a full tag object stored in the object database, containing the tagger's identity, timestamp, and message. The tag reference in `refs/tags/` points to the tag object hash, not the commit hash directly — matching Git's annotated tag format exactly.

---

### File Operations

#### `kitcat mv <src> <dst>`

Moves or renames a file or directory and updates the index. Falls back to copy-then-delete if the move crosses filesystem boundaries. All index entries under a moved directory are rewritten with their new paths.

```bash
kitcat mv old.go new.go
kitcat mv src/ lib/
kitcat mv -f old.go existing.go    # force overwrite
```

#### `kitcat rm <path>`

Removes a file from the working directory and the index.

```bash
kitcat rm file.go
kitcat rm -r src/          # recursive directory removal
kitcat rm --cached file.go # remove from index only, keep file on disk
```

#### `kitcat clean`

Removes untracked files from the working directory.

```bash
kitcat clean -f            # required force flag
kitcat clean -f -d         # also remove untracked directories
kitcat clean -f -x         # also remove ignored files
kitcat clean -n            # dry run: show what would be removed
```

---

### Search

#### `kitcat grep <pattern>`

Searches all tracked files for lines matching a regular expression. Binary files and non-UTF-8 files are skipped. Output is sorted by filename for deterministic results.

```bash
kitcat grep "TODO"
kitcat grep --line-number "func Commit"
```

---

### Listing

#### `kitcat ls-files`

Prints all paths currently in the index, sorted alphabetically.

---

### Configuration

#### `kitcat config <key> <value>`

Sets a key in the local repository config (`.kitcat/config`).

#### `kitcat config --global <key> <value>`

Sets a key in the global config (`~/.kitcatconfig`).

```bash
kitcat config user.name "Alice"
kitcat config user.email "alice@example.com"
kitcat config --global user.name "Alice"
```

#### `kitcat config --list`

Prints all key-value pairs in dot notation:

```
core.repositoryformatversion=0
user.name=Alice
user.email=alice@example.com
```

Config lookup order: local config is checked first, then global. Keys use Git's dot-separated format (`section.key` or `section.subsection.key`).

---

## Diff Engine

Diffs are computed using the **Myers algorithm** — the same algorithm used by Git, GNU diff, and most modern VCS tools. It finds the shortest edit script (SES) that transforms one sequence into another using only insertions and deletions.

kitcat's implementation in `internal/diff/myers.go`:

- Is **generic** (`MyersDiff[T comparable]`) and operates on any comparable slice
- Uses **string interning** in `DiffLines`: strings are mapped to integer IDs before the diff runs, then mapped back. This reduces comparison cost from string equality to integer equality
- Trims common prefixes and suffixes before running the core algorithm, reducing the search space
- Uses **bidirectional search** (forward and reverse simultaneously) to find the midpoint snake efficiently

**Complexity:** O(ND) time and space, where N is the total sequence length and D is the number of differences. Optimal for files with small edit distances.

---

## Merge Engine

The merge engine is split into three cleanly separated layers:

### Layer 1 — Tree Diff (`merge/engine.go`)

`MergeTrees(base, ours, theirs)` is a **pure function**: it takes three tree snapshots as maps and returns a `MergePlan` with no I/O and no side effects. This makes it trivially testable.

For each path in the union of all three trees, it evaluates one of five cases:

| Condition                | Outcome                    |
| ------------------------ | -------------------------- |
| Both match base          | Keep base version          |
| Both changed identically | Use the shared new version |
| Only ours changed        | Use our version            |
| Only theirs changed      | Use their version          |
| Both changed differently | Record as conflict         |

### Layer 2 — Text Merge (`merge/merge3.go`)

`Merge3(base, ours, theirs string)` performs line-level three-way merging for conflicting files. It runs the Myers diff independently on `base→ours` and `base→theirs`, converts those diffs into `Edit` structs (ranges in the base), then walks both edit lists in parallel:

- Non-overlapping edits from either side are applied cleanly
- Overlapping edits that produce identical output are resolved automatically
- Overlapping edits that produce different output generate `<<<<<<< HEAD` / `=======` / `>>>>>>> MERGE_HEAD` conflict markers

### Layer 3 — Application (`merge/apply.go`)

`ApplyMergePlan(plan)` executes the plan inside a `storage.UpdateIndex` transaction:

1. Deletions — remove files from disk and index
2. Clean updates — write file content from object storage, update index entry
3. Conflicts — write the text-merged file (with markers) to disk, set `Stage: 2` on the index entry to signal conflict

---

## Rebase Sequencer

The rebase sequencer follows the same design as Git's `rebase-merge` state machine.

**State files** (written to `.kitcat/rebase-merge/`):

| File              | Contents                                        |
| ----------------- | ----------------------------------------------- |
| `head-name`       | Original branch ref                             |
| `onto`            | Target branch hash                              |
| `orig-head`       | Pre-rebase HEAD hash                            |
| `git-rebase-todo` | Newline-delimited `pick <hash> <message>` list  |
| `current-step`    | Index of the current step                       |
| `stopped-sha`     | Hash of the paused commit (conflict state only) |

**Replay loop:**

For each `pick` entry, kitcat:
1. Loads the commit's parent tree as the base
2. Loads the current rebased HEAD as ours
3. Loads the commit's tree as theirs
4. Runs `MergeTrees` → `ApplyMergePlan`
5. If clean: calls `commitRebaseStep`, which creates a new commit preserving the original `author` field and setting `committer` to the current user
6. If conflict: writes `stopped-sha` and exits with a message

**Author/committer distinction:** `commitRebaseStep` intentionally preserves the original commit's author identity while setting the committer to the person running the rebase — matching Git's invariant for rebased commits.

---

## Configuration

kitcat uses Git-compatible INI-format config files:

```ini
[core]
    repositoryformatversion = 0
    filemode = false
    bare = false
    logallrefupdates = true

[user]
    name = Alice
    email = alice@example.com

[remote "origin"]
    url = https://example.com/repo
```

Two scopes are supported:

| Scope  | Location          |
| ------ | ----------------- |
| Local  | `.kitcat/config`  |
| Global | `~/.kitcatconfig` |

Local values take precedence over global values. Subsections (`[remote "origin"]`) are supported in both read and write paths.

---

## Ignore Rules

kitcat reads `.kitignore` from the repository root. Patterns follow `.gitignore` conventions:

- Lines starting with `#` are comments
- Blank lines are ignored
- Patterns ending with `/` match directories and all their contents
- `*` matches any sequence of characters within a path component
- `**` matches across directory boundaries
- Patterns are matched against both the full path and the filename

Tracked files are always exempt from ignore rules — a tracked file will never be ignored even if it matches a pattern in `.kitignore`.

Patterns are parsed once and cached in memory with a `sync.RWMutex`. Call `ClearIgnoreCache()` to force a re-read if the file changes during a long-running process.

---

## CI Automation

### Build and smoke test

`.ci/run_kitcat.sh` builds the binary and runs a basic init/config sequence:

```bash
go build -o kitcat ./cmd/main.go
mkdir -p test-repo && cd test-repo
../kitcat init
../kitcat config --global user.name "testci"
../kitcat config --global user.email "testci@example.com"
```

### Issue assignment bot

`.ci/issue-assign.js` is a GitHub Actions script for community contribution management. It responds to `/assign` and `/unassign` comments on issues:

- `/assign` — assigns the commenter if the issue has an `approved` label and no current assignee
- `/unassign` — removes the commenter's assignment if they are the current assignee

Bot comments are ignored to prevent loops.

---

## Known Limitations

**Single-parent commits only.** The `models.Commit` struct holds one `Parent string`. Merge commits are created with two parents in the object database, but the commit parser only retains the last `parent` line it reads. History traversal and merge-base computation follow only one parent chain.

**Non-transactional checkout.** If a write fails partway through a checkout, the working directory may be left in a partially-applied state with no automatic rollback.

**Index conflict representation.** Git's index supports three stages per path (base, ours, theirs) during conflicts. kitcat's index is a `map[string]IndexEntry`, so only one entry per path is possible. Conflicted files are stored with `Stage: 2` (ours) as a signal, but the base and theirs versions are not retained in the index.

**Merge-base with merge commits.** `FindMergeBase` performs a linear ancestry walk following single parents. It does not compute the true LCA for DAGs produced by previous merges.

**No pack file support.** Objects are stored as individual loose files. Large repositories with many objects will accumulate many files in `.kitcat/objects/`.

**Diff output format.** `kitcat diff` prints colored `+`/`-` lines without the standard unified diff header (`--- a/file`, `+++ b/file`, `@@ -start,count +start,count @@`).

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contribution guide, including how to set up the development environment, submit a pull request, and use the issue assignment bot.

Issue templates are available for bug reports, feature requests, refactoring proposals, and test additions.

---

## License

MIT License — see [LICENSE](LICENSE) for details.# kitcat

A Git-compatible version control system built from scratch in Go. kitcat implements Git's core object model — content-addressed storage, a binary index, zlib-compressed objects, and a full porcelain command set — using the same on-disk format as Git itself.

```
$ kitcat init
$ kitcat add .
$ kitcat commit -m "initial commit"
$ kitcat log --oneline
```

---

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Architecture](#architecture)
- [Object Model](#object-model)
- [Index (Staging Area)](#index-staging-area)
- [Command Reference](#command-reference)
- [Diff Engine](#diff-engine)
- [Merge Engine](#merge-engine)
- [Rebase Sequencer](#rebase-sequencer)
- [Configuration](#configuration)
- [Ignore Rules](#ignore-rules)
- [CI Automation](#ci-automation)
- [Contributing](#contributing)

---

## Overview

kitcat is a ground-up reimplementation of Git's fundamental mechanics. It stores objects in the same SHA-1 content-addressed format, uses the same zlib compression, reads and writes the same INI-format config files, and produces the same commit object layout — down to the `tree`, `parent`, `author`, and `committer` header lines.

The goal is correctness over compatibility at the margins. A kitcat repository can be renamed from `.kitcat` to `.git` and inspected with standard Git tooling, because the underlying formats match.

**What kitcat implements:**

- Full porcelain: `init`, `add`, `commit`, `status`, `log`, `diff`, `branch`, `checkout`, `merge`, `rebase`, `reset`, `stash`, `tag`, `mv`, `rm`, `ls-files`, `clean`, `grep`, `config`, `show-object`
- Git-compliant binary index with stat caching, file mode tracking, and exclusive write locking
- Content-addressed object database (blob, tree, commit, tag) with fan-out directory structure
- Myers diff algorithm with string interning for performance
- Three-way text merge engine with conflict marker generation
- Interactive rebase sequencer with conflict pause/resume
- Annotated and lightweight tags
- Reflog for HEAD and branch references
- `.kitignore` pattern matching with glob and `**` support
- Atomic file writes with Windows-safe retry logic
- Git-compliant INI config with local and global scope

---

## Installation

**Requirements:** Go 1.24+

```bash
git clone https://github.com/LeeFred3042U/kitcat
cd kitcat
go build -o kitcat ./cmd/main.go
```

Move the binary somewhere on your `$PATH`:

```bash
mv kitcat /usr/local/bin/kitcat
```

Or run it directly from the repo root after building.

---

## Architecture

kitcat is organized into distinct layers, each with a single responsibility:

```
cmd/                     CLI entry point — argument dispatch
internal/
  app/                   App-level constants (name)
  repo/                  Path constants (.kitcat/*, index, HEAD)
  models/                Commit struct
  plumbing/              Low-level object writers
    hash_object.go       SHA-1 hashing, zlib compression, object storage
    write_tree.go        Recursive tree object construction
    write_commit.go      Commit object serialization
    read_index.go        Binary index deserialization
    index_entry.go       IndexEntry type definition
    update_index.go      Index serialization
  storage/               Object database adapter
    adaptor.go           ReadObject, FindCommit, FindMergeBase, ReadCommits
    index.go             LoadIndex, UpdateIndex, WriteIndexFromTree
    tree.go              ParseTree (binary + legacy format), CreateTree
    stash_store.go       Stash stack push/pop/list
    safe_io.go           File I/O helpers
    lockUnix.go          Unix file locking
    lockOther.go         Windows file locking (spin-lock)
  diff/
    myers.go             Generic Myers diff algorithm
  merge/
    engine.go            MergeTrees — pure 3-way tree diff
    merge3.go            Merge3 — line-level 3-way text merge
    apply.go             ApplyMergePlan — disk writes and index update
  core/                  Porcelain commands
    add.go               AddFile, AddAll, stageFile
    branch.go            CreateBranch, DeleteBranch, ListBranches, RenameBranch
    checkout.go          Checkout, CheckoutFile
    clean.go             Clean
    commit.go            Commit, CommitAll, AmendCommit
    config.go            GetConfig, SetConfig, PrintAllConfig
    diff.go              Diff (staged and unstaged)
    grep.go              Grep
    helpers.go           SafeWrite, FindRepoRoot, ReflogAppend, ...
    ignore.go            LoadIgnorePatterns, ShouldIgnore
    index.go             Legacy JSON index (kept for compatibility)
    init.go              Init
    log.go               Log
    ls.go                ListFiles
    merge.go             Merge, MergeAbort
    move.go              MoveFile
    rebase.go            Rebase, RebaseContinue, RebaseAbort
    rebase_state.go      Sequencer state persistence
    remove.go            RemoveFile
    reset.go             Reset, CheckoutTree, ReadTree
    show.go              ShowObject
    stash.go             Stash, StashPush, StashPop, StashApply, StashDrop, StashList, StashClear
    status.go            Status, detectRenames
    tag.go               CreateTag, CreateAnnotatedTag, PrintTags
  testutil/
    setup.go             Test helper utilities
```

**Data flow for a commit:**

```
kitcat add file.go
  → stageFile() hashes content → plumbing.HashAndWriteObject() → .kitcat/objects/xx/...
  → storage.UpdateIndex() acquires lock → writes binary .kitcat/index

kitcat commit -m "msg"
  → plumbing.WriteTree() reads index → recursively builds tree objects
  → plumbing.CommitTree() writes commit object
  → updateHead() writes branch ref → ReflogAppend()
```

---

## Object Model

kitcat stores all repository data as immutable, content-addressed objects in `.kitcat/objects/`. Each object is stored at a path determined by its SHA-1 hash, using a two-level fan-out:

```
.kitcat/objects/
  3a/           ← first two hex chars
    f4b9...     ← remaining 38 chars
```

Every object is stored as:

```
<type> <size>\0<payload>
```

compressed with zlib. The SHA-1 hash is computed over the full header+payload, ensuring that identical content always produces identical hashes and that objects are de-duplicated automatically.

**Object types:**

| Type     | Description                                                           |
| -------- | --------------------------------------------------------------------- |
| `blob`   | Raw file content                                                      |
| `tree`   | Directory snapshot: `<mode> <name>\0<20-byte hash>` per entry         |
| `commit` | Points to a tree hash; contains author, committer, timestamp, message |
| `tag`    | Annotated tag object: points to a commit hash with tagger identity    |

**Commit object format:**

```
tree <tree-hash>
parent <parent-hash>
author Name <email> <unix-timestamp> <tz-offset>
committer Name <email> <unix-timestamp> <tz-offset>

<commit message>
```

This is identical to Git's commit object format, meaning kitcat commits can be read by `git cat-file` after renaming the repo directory.

---

## Index (Staging Area)

The staging area is stored as a binary file at `.kitcat/index`. Each entry records:

| Field       | Description                                |
| ----------- | ------------------------------------------ |
| `Path`      | Repo-relative file path                    |
| `Hash`      | `[20]byte` raw SHA-1 of blob               |
| `Mode`      | Unix file mode (`0100644`, `0100755`)      |
| `Size`      | File size in bytes (used for stat caching) |
| `MTimeSec`  | Modification time seconds                  |
| `MTimeNSec` | Modification time nanoseconds              |
| `Stage`     | Conflict stage (0 = clean, 2 = ours)       |

**Stat caching:** When staging a file, if the existing index entry's size and mtime match the current file's stat, hashing is skipped entirely. This avoids redundant I/O on large unchanged files.

**Exclusive locking:** All index mutations go through `storage.UpdateIndex()`, which acquires a file lock before reading, passes the in-memory map to a callback, then writes the result back. Concurrent writers are serialized; concurrent readers are not blocked.

---

## Command Reference

### Repository Setup

#### `kitcat init`

Initializes a new repository in the current directory. Creates the `.kitcat/` directory structure:

```
.kitcat/
  HEAD           → ref: refs/heads/main
  config         → INI format
  index          → binary index (empty)
  objects/       → object database
  refs/
    heads/       → branch references
    tags/        → tag references
  hooks/         → sample hook scripts
  info/exclude   → local ignore rules
```

Safe to run on an existing repository — existing files are never overwritten.

---

### Staging

#### `kitcat add <path>`

Stages a file or directory. When a directory is given, files are staged recursively while respecting `.kitignore` rules and skipping `.kitcat/` itself.

```bash
kitcat add file.go
kitcat add src/
kitcat add .
```

#### `kitcat add --all` / `kitcat add -A`

Stages all changes in the repository: new files, modifications, and deletions. Previously tracked files that no longer exist on disk are removed from the index.

---

### Committing

#### `kitcat commit -m "<message>"`

Creates a new commit from the current index state. If the repository is in a merge state (MERGE_HEAD exists), the merge commit is finalized automatically with two parents.

```bash
kitcat commit -m "add login flow"
```

#### `kitcat commit` (no message)

Opens your `$EDITOR` (or `$GIT_EDITOR`, falling back to `vi`) to compose the commit message. Lines starting with `#` are stripped. An empty message aborts the commit.

#### `kitcat commit -am "<message>"`

Stages all tracked changes (`add -A`) before committing in a single step.

#### `kitcat commit --amend`

Replaces the most recent commit with a new commit object. The current index state becomes the new commit's tree. Cannot be used during an active merge.

---

### Inspecting State

#### `kitcat status`

Shows the current branch and three sections of changes:

- **Unmerged paths** — files with active conflict markers
- **Changes to be committed** — staged changes (index vs HEAD)
- **Changes not staged for commit** — unstaged changes (working tree vs index)
- **Untracked files** — files not in the index

Rename detection is performed in the staged section using two phases: exact hash match first, then Jaccard line-similarity scoring (>50% threshold) for approximate matches.

#### `kitcat log`

Displays the full commit history from HEAD, following parent pointers. Each entry shows the commit hash, author, date, and message.

```bash
kitcat log
kitcat log --oneline     # compact single-line format
kitcat log -n 5          # limit to 5 commits
```

#### `kitcat diff`

Shows unstaged changes: working directory vs index.

#### `kitcat diff --staged`

Shows staged changes: index vs HEAD.

Both modes use the Myers diff algorithm and print colored `+`/`-` output per line.

#### `kitcat show-object <hash>`

Prints the raw decompressed payload of any object in the database. Useful for inspecting blobs, trees, and commits directly.

---

### Branching

#### `kitcat branch`

Lists all local branches. The current branch is highlighted with a `*`.

#### `kitcat branch <name>`

Creates a new branch pointing to the current HEAD commit.

#### `kitcat branch -m <new-name>`

Renames the currently checked-out branch. Updates HEAD, the ref file, and writes reflog entries for both the old and new names.

#### `kitcat branch -d <name>`

Deletes a branch. Rejected if the branch is currently checked out.

---

### Switching

#### `kitcat checkout <branch>`

Switches to an existing branch or detached commit hash. Before switching:

1. Aborts if the working directory has uncommitted changes (unless `-f` is passed)
2. Checks for untracked files that would be overwritten by files in the target tree

On success, updates the working directory, rebuilds the index inside a transaction, and moves HEAD last. A reflog entry is written.

```bash
kitcat checkout main
kitcat checkout abc1234    # detached HEAD
kitcat checkout -f main    # force, discard local changes
```

#### `kitcat checkout -b <name>`

Creates a new branch and immediately switches to it.

#### `kitcat checkout -- <file>`

Restores a single file from the last commit into the working directory. Refuses to overwrite a file with local modifications or an untracked file that would be clobbered.

---

### Merging

#### `kitcat merge <branch>`

Merges the specified branch into the current branch.

**Fast-forward merge:** If the current HEAD is an ancestor of the target, the branch pointer is simply advanced and the workspace is updated. No merge commit is created.

**Three-way merge:** If the branches have diverged, kitcat computes the merge base, then runs a three-way merge across all affected files:

- Files unchanged by both branches: kept as-is
- Files changed by only one branch: that version wins automatically
- Files changed identically by both branches: that version wins automatically
- Files changed differently by both branches: conflict markers are written

After a three-way merge, `MERGE_HEAD` and `MERGE_MSG` are written to `.kitcat/`. Run `kitcat commit` to finalize the merge commit, or `kitcat merge --abort` to cancel.

**Conflict markers:**

```
<<<<<<< HEAD
ours version of the line
=======
their version of the line
>>>>>>> MERGE_HEAD
```

#### `kitcat merge --abort`

Cancels an active merge, hard-resets back to the pre-merge HEAD, and removes `MERGE_HEAD` and `MERGE_MSG`.

---

### Rebasing

#### `kitcat rebase <branch>`

Replays commits from the current branch onto the tip of the target branch. For each commit to replay:

1. The commit's parent tree is used as the merge base
2. A three-way merge is computed between that base, the current rebased HEAD, and the commit's tree
3. If clean, a new commit is created preserving the original author and timestamp, with the current user as committer
4. If conflicts occur, the sequencer pauses and writes the conflicting commit hash to `rebase-merge/stopped-sha`

```bash
kitcat rebase main
```

#### `kitcat rebase -i <branch>`

Interactive rebase. Opens the todo list in `$EDITOR`. Supported actions:

| Action       | Description       |
| ------------ | ----------------- |
| `pick` / `p` | Apply the commit  |
| `drop` / `d` | Remove the commit |

Removing a line also drops the commit.

#### `kitcat rebase --continue`

After resolving conflicts with `kitcat add`, continues the sequencer from where it paused.

#### `kitcat rebase --abort`

Cancels the rebase and restores HEAD to its pre-rebase state.

---

### Resetting

#### `kitcat reset --soft <commit>`

Moves the branch pointer to the specified commit. Index and working directory are untouched.

#### `kitcat reset --mixed <commit>` (default)

Moves the branch pointer and rebuilds the index to match the target commit. Working directory is untouched.

#### `kitcat reset --hard <commit>`

Moves the branch pointer, rebuilds the index, and overwrites the working directory to exactly match the target commit. Discards all local changes.

```bash
kitcat reset --hard HEAD
kitcat reset --hard abc1234
kitcat reset --hard main
```

---

### Stashing

#### `kitcat stash`

Saves the current working directory and index state as a WIP commit, then hard-resets to HEAD. The stash is pushed onto a stack stored at `.kitcat/refs/stash`.

```bash
kitcat stash
kitcat stash push -m "work in progress on auth"
```

#### `kitcat stash list`

Shows all stashed states, newest first:

```
stash@{0}: WIP on main: abc1234 last commit message
stash@{1}: WIP on feature: def5678 ...
```

#### `kitcat stash pop`

Applies the most recent stash to the working directory and removes it from the stack. Aborts if the working directory is dirty.

#### `kitcat stash apply [<index>]`

Applies the stash at the given index without removing it from the stack.

#### `kitcat stash drop [<index>]`

Removes a stash entry without applying it.

#### `kitcat stash clear`

Removes all stash entries.

---

### Tagging

#### `kitcat tag`

Lists all tags.

#### `kitcat tag <name> <commit>`

Creates a lightweight tag: a named reference pointing directly to a commit hash.

#### `kitcat tag -a -m "<message>" <name> <commit>`

Creates an annotated tag: a full tag object stored in the object database, containing the tagger's identity, timestamp, and message. The tag reference in `refs/tags/` points to the tag object hash, not the commit hash directly — matching Git's annotated tag format exactly.

---

### File Operations

#### `kitcat mv <src> <dst>`

Moves or renames a file or directory and updates the index. Falls back to copy-then-delete if the move crosses filesystem boundaries. All index entries under a moved directory are rewritten with their new paths.

```bash
kitcat mv old.go new.go
kitcat mv src/ lib/
kitcat mv -f old.go existing.go    # force overwrite
```

#### `kitcat rm <path>`

Removes a file from the working directory and the index.

```bash
kitcat rm file.go
kitcat rm -r src/          # recursive directory removal
kitcat rm --cached file.go # remove from index only, keep file on disk
```

#### `kitcat clean`

Removes untracked files from the working directory.

```bash
kitcat clean -f            # required force flag
kitcat clean -f -d         # also remove untracked directories
kitcat clean -f -x         # also remove ignored files
kitcat clean -n            # dry run: show what would be removed
```

---

### Search

#### `kitcat grep <pattern>`

Searches all tracked files for lines matching a regular expression. Binary files and non-UTF-8 files are skipped. Output is sorted by filename for deterministic results.

```bash
kitcat grep "TODO"
kitcat grep --line-number "func Commit"
```

---

### Listing

#### `kitcat ls-files`

Prints all paths currently in the index, sorted alphabetically.

---

### Configuration

#### `kitcat config <key> <value>`

Sets a key in the local repository config (`.kitcat/config`).

#### `kitcat config --global <key> <value>`

Sets a key in the global config (`~/.kitcatconfig`).

```bash
kitcat config user.name "Alice"
kitcat config user.email "alice@example.com"
kitcat config --global user.name "Alice"
```

#### `kitcat config --list`

Prints all key-value pairs in dot notation:

```
core.repositoryformatversion=0
user.name=Alice
user.email=alice@example.com
```

Config lookup order: local config is checked first, then global. Keys use Git's dot-separated format (`section.key` or `section.subsection.key`).

---

## Diff Engine

Diffs are computed using the **Myers algorithm** — the same algorithm used by Git, GNU diff, and most modern VCS tools. It finds the shortest edit script (SES) that transforms one sequence into another using only insertions and deletions.

kitcat's implementation in `internal/diff/myers.go`:

- Is **generic** (`MyersDiff[T comparable]`) and operates on any comparable slice
- Uses **string interning** in `DiffLines`: strings are mapped to integer IDs before the diff runs, then mapped back. This reduces comparison cost from string equality to integer equality
- Trims common prefixes and suffixes before running the core algorithm, reducing the search space
- Uses **bidirectional search** (forward and reverse simultaneously) to find the midpoint snake efficiently

**Complexity:** O(ND) time and space, where N is the total sequence length and D is the number of differences. Optimal for files with small edit distances.

---

## Merge Engine

The merge engine is split into three cleanly separated layers:

### Layer 1 — Tree Diff (`merge/engine.go`)

`MergeTrees(base, ours, theirs)` is a **pure function**: it takes three tree snapshots as maps and returns a `MergePlan` with no I/O and no side effects. This makes it trivially testable.

For each path in the union of all three trees, it evaluates one of five cases:

| Condition                | Outcome                    |
| ------------------------ | -------------------------- |
| Both match base          | Keep base version          |
| Both changed identically | Use the shared new version |
| Only ours changed        | Use our version            |
| Only theirs changed      | Use their version          |
| Both changed differently | Record as conflict         |

### Layer 2 — Text Merge (`merge/merge3.go`)

`Merge3(base, ours, theirs string)` performs line-level three-way merging for conflicting files. It runs the Myers diff independently on `base→ours` and `base→theirs`, converts those diffs into `Edit` structs (ranges in the base), then walks both edit lists in parallel:

- Non-overlapping edits from either side are applied cleanly
- Overlapping edits that produce identical output are resolved automatically
- Overlapping edits that produce different output generate `<<<<<<< HEAD` / `=======` / `>>>>>>> MERGE_HEAD` conflict markers

### Layer 3 — Application (`merge/apply.go`)

`ApplyMergePlan(plan)` executes the plan inside a `storage.UpdateIndex` transaction:

1. Deletions — remove files from disk and index
2. Clean updates — write file content from object storage, update index entry
3. Conflicts — write the text-merged file (with markers) to disk, set `Stage: 2` on the index entry to signal conflict

---

## Rebase Sequencer

The rebase sequencer follows the same design as Git's `rebase-merge` state machine.

**State files** (written to `.kitcat/rebase-merge/`):

| File              | Contents                                        |
| ----------------- | ----------------------------------------------- |
| `head-name`       | Original branch ref                             |
| `onto`            | Target branch hash                              |
| `orig-head`       | Pre-rebase HEAD hash                            |
| `git-rebase-todo` | Newline-delimited `pick <hash> <message>` list  |
| `current-step`    | Index of the current step                       |
| `stopped-sha`     | Hash of the paused commit (conflict state only) |

**Replay loop:**

For each `pick` entry, kitcat:
1. Loads the commit's parent tree as the base
2. Loads the current rebased HEAD as ours
3. Loads the commit's tree as theirs
4. Runs `MergeTrees` → `ApplyMergePlan`
5. If clean: calls `commitRebaseStep`, which creates a new commit preserving the original `author` field and setting `committer` to the current user
6. If conflict: writes `stopped-sha` and exits with a message

**Author/committer distinction:** `commitRebaseStep` intentionally preserves the original commit's author identity while setting the committer to the person running the rebase — matching Git's invariant for rebased commits.

---

## Configuration

kitcat uses Git-compatible INI-format config files:

```ini
[core]
    repositoryformatversion = 0
    filemode = false
    bare = false
    logallrefupdates = true

[user]
    name = Alice
    email = alice@example.com

[remote "origin"]
    url = https://example.com/repo
```

Two scopes are supported:

| Scope  | Location          |
| ------ | ----------------- |
| Local  | `.kitcat/config`  |
| Global | `~/.kitcatconfig` |

Local values take precedence over global values. Subsections (`[remote "origin"]`) are supported in both read and write paths.

---

## Ignore Rules

kitcat reads `.kitignore` from the repository root. Patterns follow `.gitignore` conventions:

- Lines starting with `#` are comments
- Blank lines are ignored
- Patterns ending with `/` match directories and all their contents
- `*` matches any sequence of characters within a path component
- `**` matches across directory boundaries
- Patterns are matched against both the full path and the filename

Tracked files are always exempt from ignore rules — a tracked file will never be ignored even if it matches a pattern in `.kitignore`.

Patterns are parsed once and cached in memory with a `sync.RWMutex`. Call `ClearIgnoreCache()` to force a re-read if the file changes during a long-running process.

---

## CI Automation

### Build and smoke test

`.ci/run_kitcat.sh` builds the binary and runs a basic init/config sequence:

```bash
go build -o kitcat ./cmd/main.go
mkdir -p test-repo && cd test-repo
../kitcat init
../kitcat config --global user.name "testci"
../kitcat config --global user.email "testci@example.com"
```

### Issue assignment bot

`.ci/issue-assign.js` is a GitHub Actions script for community contribution management. It responds to `/assign` and `/unassign` comments on issues:

- `/assign` — assigns the commenter if the issue has an `approved` label and no current assignee
- `/unassign` — removes the commenter's assignment if they are the current assignee

Bot comments are ignored to prevent loops.

---

## Known Limitations

**Single-parent commits only.** The `models.Commit` struct holds one `Parent string`. Merge commits are created with two parents in the object database, but the commit parser only retains the last `parent` line it reads. History traversal and merge-base computation follow only one parent chain.

**Non-transactional checkout.** If a write fails partway through a checkout, the working directory may be left in a partially-applied state with no automatic rollback.

**Index conflict representation.** Git's index supports three stages per path (base, ours, theirs) during conflicts. kitcat's index is a `map[string]IndexEntry`, so only one entry per path is possible. Conflicted files are stored with `Stage: 2` (ours) as a signal, but the base and theirs versions are not retained in the index.

**Merge-base with merge commits.** `FindMergeBase` performs a linear ancestry walk following single parents. It does not compute the true LCA for DAGs produced by previous merges.

**No pack file support.** Objects are stored as individual loose files. Large repositories with many objects will accumulate many files in `.kitcat/objects/`.

**Diff output format.** `kitcat diff` prints colored `+`/`-` lines without the standard unified diff header (`--- a/file`, `+++ b/file`, `@@ -start,count +start,count @@`).

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contribution guide, including how to set up the development environment, submit a pull request, and use the issue assignment bot.

Issue templates are available for bug reports, feature requests, refactoring proposals, and test additions.

---

## License

MIT License — see [LICENSE](LICENSE) for details.
