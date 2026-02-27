# Contributing to kitcat

Thank you for your interest in contributing to **kitcat**! We are building an educational reimplementation of Git in Go, and we welcome contributors of all experience levels.

Whether you are a student looking for your first open-source contribution or a systems engineer wanting to solve complex algorithmic challenges, there is a place for you here.

## Join Our Community

Have questions? Want to discuss ideas or get help? Join our Discord server: **https://discord.gg/x6henXZs**

## Ways to Contribute

We have organized contributions into three tracks. Please choose one that matches your interest:

## Pick an Issue Difficulty

> Note:
> These tracks describe **issue difficulty and learning scope only**.
> They do **not** define Pull Request structure, review standards, or change risk.
> PR review is based on the _type of change_ (feature, fix, test, chore), not the issue difficulty label.

### Easy (Start Here)

**Labels:** `Easy` `documentation` `good first issue`
Generally suitable if you are new to Go or Open Source

- **What you do:** Fix typos, add simple CLI commands, update `README.md`
- **Example:** "Fix the help message for `rm`"

### Medium (The Real Work)

**Labels:** `Medium` `bug` `core`
Great if you know some Go and want to build features.

- **What you do:** Add new logic, fix standard bugs, handle flags.
- **Example:** "Implement `kitcat log -n 5`"

### Hard (Core Logic)

**Labels:** `Hard` `core`
For contributors comfortable reasoning about complex behavior

- **What you do:** Graph traversal, file locking, hashing, binary formats
- **Example:** "Implement `reset --hard` with tree traversal"

> Note: This is not applied everytime

---

## Workflow (How to Contribute)

- Go to the Issues tab or click [this](https://github.com/LeeFred3042U/kitcat/issues). Look for a label (Easy, Medium, etc)
- Comment: "/assign"
- You will be automatically assigned the issue by a bot
- You cant be working on multiple issue at a time
- When your pr is merged only then you may work on a new issue

### Prerequisites

- **Go 1.24+** installed (Check `go.mod` for the exact version)
- A text editor (VS Code and vim recommended)

## Setup

### 1. Install Go

- Make sure you have **Go 1.24+**.
- Check with: `go version`

### 2. Fork and Clone

1. **Fork** this repo (Click the button top-right)
   - it looks like this

   - ![alt text](assets/image-1.png)

2. **Clone** your fork:

```bash
git clone https://github.com/username/kitcat.git
cd kitcat
```

### 3. Add Upstream Remote

To keep your fork synchronized with the main repository, add the original repository as an upstream remote:

```bash
git remote add upstream https://github.com/LeeFred3042U/kitcat.git
```

Verify your remotes:

```bash
git remote -v
```

You should see:

```
origin    https://github.com/username/kitcat.git (fetch)
origin    https://github.com/username/kitcat.git (push)
upstream  https://github.com/LeeFred3042U/kitcat.git (fetch)
upstream  https://github.com/LeeFred3042U/kitcat.git (push)
```

### 4. Sync Your Fork

Before starting work on a new feature, always sync your fork with upstream:

```bash
# Switch to your main branch
git checkout main

# Pull latest changes from upstream
git pull upstream main

# Push updates to your fork
git push origin main
```

> **Pro Tip:** Run these commands regularly to stay up to date and avoid merge conflicts!

### 5. Create a Branch

- Use a descriptive name for your branch
- Do not work on main
- Make a new branch from the updated main

```bash
git checkout -b feat/implement-rm-command
# or
git checkout -b docs/add-status-diagram
```

### 6. Build the Project

```bash
go build -o kitcat ./cmd/main.go
```

### 7. Verify if it Runs

```bash
./kitcat init
./kitcat help
```

### 8. Make Changes

**For code:**

- Write clean, idiomatic Go code
- If you are new to Go, feel free to ask for help in the PR or on our [Discord server](https://discord.gg/x6henXZs)!

**For documentation:**

- Work as stated in the issue
- Keep check for typos

### 9. Test

Manual testing is required

- Please include (if code changes were made, else no need) a **screenshot** or **terminal output** or **screen recording** in your Pull Request description proving the command works as expected
- Run `go fmt ./...` before you commit, else we have issues

### Before opening a Pull Request

Before you open a PR, do these two things

1. Sync with `main`:
   - Run: `git fetch origin` then either:
     - `git rebase origin/main` (preferred) or
     - `git merge origin/main`
   - Confirm: `git rev-parse --abbrev-ref HEAD` shows your feature branch and `git rev-list --left-right --count origin/main...HEAD` shows your branch is up to date with or ahead of `origin/main`.
   - If you do not sync with `main`, your PR will be closed with the instruction to rebase/merge first.

2. Squash commits for PRs targeting `main`:
   - If your PR **targets `main`**, it must contain **exactly one commit**
     - Squash locally: `git rebase -i origin/main` and squash into one commit
     - Force-push: `git push --force-with-lease origin main`

> NOTE: We enforce this manually - do not open a PR to `main` with multiple commits

### Push & PR

- Push your branch to your fork:

```bash
git push origin feat/implement-rm-command
```

- Go to GitHub and open a Pull Request
- Keep the description concise, and reference the issue number (e.g., `Fixes #1`)
- The title should be named as the issue title which is fixed by you

## Pull Request Verification Standard (MANDATORY) [only for code changes]

We require **Proof of Work** for every Pull Request
"It works on my machine" is not enough
You must include a **Screenshot** or **Terminal Output** in your PR description showing the command running successfully.

**Acceptable Example (Terminal Output):**

> I tested the `help` command. Here is the output of terminal showing it

```bash
[terminal@terminal kitcat] $ ./kitcat help
usage: kitcat <command> [arguments]

These are the common KitCat commands:
   tag        Create a new tag for a commit
   merge      Merge a branch into the current branch.
   ls-files   Show information about files in the index
   config     Get and set repository or global options.
   commit     Record changes to the repository.
   log        Show the commit history
   clean      Remove untracked files from the working directory
   init       Initialize a new KitCat repository
   add        Add file contents to the index.
   diff       Show changes between the last commit and staging area

Use 'kitcat help <command>' for more information about a command
```

OR

> I tested `add` and `commit` command(since both go together). Here is the output of terminal shown in a screenshot

![alt text](assets/image.png)

---

# Editing & Creating Architecture Diagrams (PlantUML)

KitCat's architecture diagrams are stored in `.puml` format and exported as `.png`.
If you are **creating new diagrams**, follow the same workflow used for editing existing ones.

All source files live under:

```
docs/architecture/<section>/
```

Each diagram should always consist of:

```
diagram-name.puml   (source file)
diagram-name.png    (exported image checked into the repo)
```

## Required Tool (VS Code)

We use the following extension so contributors can preview and export diagrams:

- **Name:** PlantUML Viewer
- **ID:** `BenkoSoftware.plantumlviewer`
- **Publisher:** BenkoSoftware
- **Version:** 1.1.0

## How to Use It

1. Install the extension
2. Press `Ctrl + Shift + P`, search **PlantUML**
3. Add keybindings for(because it makes it easier):

- _Open Preview_
- _Export as PNG_

## Workflow for New or Updated Diagrams

1. Create or edit the `.puml` file
2. Open the preview to confirm the diagram renders correctly
3. Export to PNG
4. Commit both files inside the architecture directory following this structure:

```
docs/
└── architecture/
    └── <section-name>/
        ├── <diagram-name>.puml
        └── <diagram-name>.png
```

Pull Requests missing the PNG export will be rejected.

---

## Security Reporting

Please refer to [SECURITY.md](SECURITY.md) for details. Issues involving **data loss, repository corruption, or checkout/reset overwrites** must be reported privately via the email listed in `SECURITY.md` and **must NOT** be reported via public GitHub issues or Pull Requests

---

## Code of Conduct

Please note that this project is released with a [Code of Conduct](./CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.
