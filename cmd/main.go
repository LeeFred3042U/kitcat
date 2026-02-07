package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/models"
)

type CommandFunc func(args []string)

var commands = map[string]CommandFunc{
	"init": func(args []string) {
		core.EnsureArgs(args, 0, 0, "init")
		if err := core.InitRepo(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	},
	"add": func(args []string) {
		if len(args) < 1 {
			fmt.Println("Usage: kitcat add <file-path>")
			os.Exit(2)
		}
		if args[0] == "-A" || args[0] == "--all" {
			fmt.Println("Staging all changes...")
			if err := core.AddAll(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
		exitCode := 0
		for _, path := range args {
			if err := core.AddFile(path); err != nil {
				fmt.Printf("Error adding %s: %v\n", path, err)
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	},
	"grep": func(args []string) {
		if err := core.Grep(args); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		os.Exit(0)
	},

	"rm": func(args []string) {
		if len(args) < 1 {
			fmt.Println("Usage: kitcat rm [-r] <file>")
			os.Exit(2)
		}

		recursive := false
		filesToRemove := []string{}

		i := 0
		for i < len(args) {
			switch args[i] {
			case "-r":
				recursive = true
				i++
			default:
				filesToRemove = append(filesToRemove, args[i])
				i++
			}
		}

		if len(filesToRemove) == 0 {
			fmt.Println("Usage: kitcat rm [-r] <file>")
			os.Exit(2)
		}

		exitCode := 0
		for _, filename := range filesToRemove {
			if err := core.RemoveFile(filename, recursive); err != nil {
				fmt.Println("Error:", err)
				exitCode = 1
			}
		}
		os.Exit(exitCode)
	},
	"commit": func(args []string) {
		if !core.IsRepoInitialized() {
			fmt.Println(
				"Error: not a kitcat repository (or any of the parent directories): .kitcat",
			)
			os.Exit(1)
		}

		if len(args) < 2 {
			fmt.Println("Usage: kitcat commit <-m | -am | --amend> <message>")
			os.Exit(2)
		}

		var isAmend bool
		var message string

		switch args[0] {
		// Checks for amending
		case "--amend":
			if len(args) < 3 || args[1] != "-m" {
				fmt.Println("Usage: kitcat commit --amend -m <message>")
				os.Exit(2)
			}
			isAmend = true
			message = strings.Join(args[2:], " ")
		// Normal commit flow
		case "-am":
			message = strings.Join(args[1:], " ")
			newCommit, summary, err := core.CommitAll(message)
			if err != nil {
				if err.Error() == "nothing to commit, working tree clean" {
					fmt.Println(err.Error())
					os.Exit(1)
				}
				os.Exit(2)
			}
			printCommitResult(newCommit, summary)
			os.Exit(0)
		case "-m":
			message = strings.Join(args[1:], " ")
		default:
			fmt.Println("Usage: kitcat commit <-m | -am | --amend> <message>")
			os.Exit(2)
		}

		// Handle amend or normal commit
		if isAmend {
			newCommit, err := core.AmendCommit(message)
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			headState, err := core.GetHeadState()
			if err != nil {
				headData, _ := os.ReadFile(".kitcat/HEAD")
				ref := strings.TrimSpace(string(headData))
				headState = strings.TrimPrefix(ref, "ref: refs/heads/")
			}
			fmt.Printf("[%s %s] %s (amended)\n", headState, newCommit.ID[:7], newCommit.Message)
			os.Exit(0)
		} else {
			newCommit, summary, err := core.Commit(message)
			if err != nil {
				if err.Error() == "nothing to commit, working tree clean" {
					fmt.Println(err.Error())
					os.Exit(1)
				} else {
					fmt.Println("Error:", err)
					os.Exit(1)
				}
			}
			printCommitResult(newCommit, summary)
			os.Exit(0)
		}
	},
	"log": func(args []string) {
		oneline := false
		limit := -1
		i := 0
		for i < len(args) {
			switch args[i] {
			case "--oneline":
				oneline = true
				i++
			case "-n":
				if i+1 >= len(args) {
					fmt.Println("Error: -n requires a positive integer argument")
					os.Exit(2)
				}
				var n int
				_, err := fmt.Sscanf(args[i+1], "%d", &n)
				if err != nil || n <= 0 {
					fmt.Println("Error: -n requires a positive integer argument")
					os.Exit(2)
				}
				limit = n
				i += 2
			default:
				fmt.Printf("Error: unknown flag %s\n", args[i])
				os.Exit(2)
			}
		}
		if err := core.ShowLog(oneline, limit); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	},
	"shortlog": func(args []string) {
		if err := core.ShowShortLog(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	},
	"status": func(args []string) {
		if err := core.Status(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	},
	"diff": func(args []string) {
		staged := false
		stat := false
		for _, arg := range args {
			switch arg {
			case "--cached", "--staged":
				staged = true
			case "--stat":
				stat = true
			default:
				fmt.Println("Path filtering not supported")
				os.Exit(2)
			}
		}
		if err := core.Diff(staged, stat); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	},
	"checkout": func(args []string) {
		if len(args) < 1 {
			fmt.Println(
				"Usage: kitcat checkout [-b] <branch-name> | <file-path> | <branch> -- <file-path>",
			)
			os.Exit(2)
		}

		// Handle branch creation: kitcat checkout -b <branch-name>
		if args[0] == "-b" {
			if len(args) != 2 {
				fmt.Println("Usage: kitcat checkout -b <branch-name>")
				os.Exit(2)
			}
			name := args[1]
			if core.IsBranch(name) {
				fmt.Printf("Error: Branch '%s' already exists\n", name)
				os.Exit(1)
			}
			if err := core.CreateBranch(name); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			if err := core.CheckoutBranch(name); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		// Support for -- separator
		sepIdx := -1
		for i, arg := range args {
			if arg == "--" {
				sepIdx = i
				break
			}
		}

		if sepIdx != -1 {
			// Arguments before --
			branchArgs := args[:sepIdx]
			// Arguments after -- are always file paths
			fileArgs := args[sepIdx+1:]
			if len(fileArgs) == 0 {
				fmt.Println("Error: No file paths provided after --")
				os.Exit(2)
			}

			// If a branch is specified before --, switch to it
			if len(branchArgs) == 1 {
				branch := branchArgs[0]
				if !core.IsBranch(branch) {
					fmt.Printf("Error: Branch '%s' does not exist\n", branch)
					os.Exit(1)
				}
				if err := core.CheckoutBranch(branch); err != nil {
					fmt.Println("Error:", err)
					os.Exit(1)
				}
			} else if len(branchArgs) > 1 {
				fmt.Println("Error: Too many arguments before --")
				os.Exit(2)
			}

			// Now restore each file path after --
			for _, file := range fileArgs {
				if _, err := os.Stat(file); err != nil {
					fmt.Printf("Error: file '%s' does not exist on disk\n", file)
					os.Exit(1)
				}
				if err := core.CheckoutFile(file); err != nil {
					fmt.Println("Error:", err)
					os.Exit(1)
				}
			}
			os.Exit(0)
		}

		// No -- separator: fallback to old logic
		name := args[0]
		if core.IsBranch(name) {
			if err := core.CheckoutBranch(name); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		} else {
			if _, err := os.Stat(name); err != nil {
				fmt.Printf("Error: file '%s' does not exist on disk\n", name)
				os.Exit(1)
			}
			if err := core.CheckoutFile(name); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		}
	},
	"merge": func(args []string) {
		if len(args) < 1 {
			fmt.Println("Usage: kitcat merge <branch-name>")
			os.Exit(2)
		}
		if err := core.Merge(args[0]); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		os.Exit(0)
	},
	"reset": func(args []string) {
		if len(args) < 2 {
			fmt.Println("Usage: kitkat reset [--soft | --mixed | --hard] <commit-hash>")
			os.Exit(2)
		}

		mode := ""
		commitHash := ""

		// Helper function to safely get next argument
		safeNext := func(i int) string {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}

		i := 0
		for i < len(args) {
			switch args[i] {
			case "--" + core.ResetSoft:
				mode = core.ResetSoft
				commitHash = safeNext(i)
				i += 2 // Skip current and next arg
			case "--" + core.ResetMixed:
				mode = core.ResetMixed
				commitHash = safeNext(i)
				i += 2 // Skip current and next arg
			case "--" + core.ResetHard:
				mode = core.ResetHard
				commitHash = safeNext(i)
				i += 2 // Skip current and next arg
			default:
				commitHash = args[i]
				mode = core.ResetMixed
				i++
			}
		}

		if mode == "" {
			fmt.Println("Error: must specify --soft, --mixed, or --hard")
			fmt.Println("Usage: kitkat reset [--soft | --mixed | --hard] <commit-hash>")
			os.Exit(2)
		}

		if commitHash == "" {
			fmt.Println("Error: commit hash required")
			os.Exit(2)
		}

		if err := core.Reset(commitHash, mode); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		os.Exit(0)
	},
	"rebase": func(args []string) {
		if len(args) < 1 {
			fmt.Println("Usage: kitcat rebase [-i <commit> | --continue | --abort]")
			os.Exit(2)
		}

		switch args[0] {
		case "--abort":
			if err := core.RebaseAbort(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "--continue":
			if err := core.RebaseContinue(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "-i":
			if len(args) < 2 {
				fmt.Println("Usage: kitcat rebase -i <commit>")
				os.Exit(2)
			}
			if err := core.RebaseInteractive(args[1]); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		default:
			// If no flag, assumes simple rebase which isn't requested but we can default to error
			fmt.Println("Usage: kitcat rebase [-i <commit> | --continue | --abort]")
			os.Exit(2)
		}
	},
	"ls-files": func(args []string) {
		core.EnsureArgs(args, 0, 0, "ls-files")
		if !core.IsRepoInitialized() {
			fmt.Println(
				"Error: not a kitcat repository (or any of the parent directories): .kitcat",
			)
			os.Exit(1)
		}

		entries, err := core.LoadIndex()
		if err != nil {
			fmt.Println("Error loading index:", err)
			os.Exit(1)
		}

		for _, entry := range entries {
			fmt.Println(entry.Path)
		}
		os.Exit(0)
	},
	"clean": func(args []string) {
		dryRun := false
		force := false
		includeIgnored := false

		for _, arg := range args {
			switch arg {
			case "-f":
				force = true
			case "-fd":
				force = true
				dryRun = true
			case "-x":
				includeIgnored = true
			}
		}

		if !force {
			fmt.Println("This will delete untracked files. Run 'kitcat clean -f' to proceed.")
			os.Exit(1)
		}

		if err := core.Clean(dryRun, includeIgnored); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		os.Exit(0)
	},
	"help": func(args []string) {
		if len(args) > 0 {
			core.PrintCommandHelp(args[0])
		} else {
			core.PrintGeneralHelp()
		}
		os.Exit(0)
	},
	"tag": func(args []string) {
		if !core.IsRepoInitialized() {
			fmt.Println(
				"Error: not a kitcat repository (or any of the parent directories): .kitcat",
			)
			os.Exit(1)
		}

		if len(args) == 1 && (args[0] == "--list") {
			if err := core.PrintTags(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		if len(args) < 2 {
			fmt.Println("Usage: kitcat tag <tag-name> <commit-id>")
			os.Exit(2)
		}

		if err := core.CreateTag(args[0], args[1]); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		os.Exit(0)
	},
	"config": func(args []string) {
		if len(args) == 0 {
			fmt.Println("Usage: kitcat config [--global] <key> [<value>]")
			os.Exit(2)
		}

		// Support listing configuration
		if len(args) == 1 && args[0] == "--list" {
			if err := core.PrintAllConfig(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		global := false
		argIndex := 0
		if args[0] == "--global" {
			global = true
			argIndex = 1
		}

		// After optional --global, we need at least a key
		if len(args) <= argIndex {
			fmt.Println("Usage: kitcat config [--global] <key> [<value>]")
			os.Exit(2)
		}

		key := args[argIndex]

		// Determine if we're setting or getting
		if len(args) == argIndex+2 {
			value := args[argIndex+1]
			if err := core.SetConfig(key, value, global); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		if len(args) == argIndex+1 {
			value, ok, err := core.GetConfig(key)
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			if ok {
				fmt.Println(value)
				os.Exit(0)
			}
			os.Exit(1)
		}

		fmt.Println("Usage: kitcat config [--global] <key> [<value>]")
		os.Exit(2)
	},
	"show-object": func(args []string) {
		if len(args) != 1 {
			fmt.Println("Usage: kitcat show-object <hash>")
			os.Exit(2)
			return
		}
		if err := core.ShowObject(args[0]); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		os.Exit(0)
	},
	"branch": func(args []string) {
		if len(args) == 0 {
			if err := core.ListBranches(); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
		switch args[0] {
		case "-l":
			if err := core.ListBranches(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "-r":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "Usage: kitcat branch -r <branch-name>")
				os.Exit(2)
			}

			name := args[1]
			if err := core.RenameCurrentBranch(name); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "-d", "--delete":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "Usage: kitcat branch -d <branch-name>")
				os.Exit(2)
			}

			name := args[1]
			if err := core.DeleteBranch(name); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			} else {
				fmt.Println("Branch `" + name + "` deleted successfully")
				os.Exit(0)
			}
		default:
			name := args[0]
			if core.IsBranch(name) {
				fmt.Printf("Error: Branch '%s' already exists\n", name)
				os.Exit(1)
			}
			if err := core.CreateBranch(name); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	},
	"mv": func(args []string) {
		force := false
		paths := make([]string, 0, 2)

		for _, arg := range args {
			if arg == "-f" || arg == "--force" {
				force = true
				continue
			}
			paths = append(paths, arg)
		}

		if len(paths) != 2 {
			fmt.Println("Usage: kitcat mv [-f|--force] <old_path> <new_path>")
			os.Exit(2)
		}

		if err := core.MoveFile(paths[0], paths[1], force); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		os.Exit(0)
	},
	"stash": func(args []string) {
		if !core.IsRepoInitialized() {
			fmt.Println(
				"Error: not a kitcat repository (or any of the parent directories): .kitcat",
			)
			os.Exit(1)
		}
		if len(args) > 0 && args[0] == "list" {
			if err := core.StashList(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		if len(args) > 0 && args[0] == "push" {
			message := ""
			if len(args) > 1 {
				message = strings.Join(args[1:], " ")
			}
			if err := core.StashPush(message); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			fmt.Println("Saved working directory and index state")
			os.Exit(0)
		}

		if len(args) > 0 && args[0] == "pop" {
			if err := core.StashPop(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		if len(args) > 1 && args[0] == "apply" {
			idx, err := parseStashIndex(args[1])
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			if err := core.StashApply(idx); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		if len(args) > 1 && args[0] == "drop" {
			idx, err := parseStashIndex(args[1])
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(2)
			}
			if err := core.StashDrop(idx); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		if len(args) > 0 && args[0] == "clear" {
			if err := core.StashClear(); err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			fmt.Println("Cleared all stash entries")
			os.Exit(0)
		}

		// Default: stash save
		if err := core.Stash(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		fmt.Println("Saved working directory and index state")
		os.Exit(0)
	},
}

// parseStashIndex parses a string index for stash commands.
func parseStashIndex(s string) (int, error) {
	var idx int
	_, err := fmt.Sscanf(s, "%d", &idx)
	if err != nil {
		return 0, fmt.Errorf("invalid stash index: %s", s)
	}
	return idx, nil
}

// printCommitResult formats and prints the commit result with summary
func printCommitResult(newCommit models.Commit, summary string) {
	headState, err := core.GetHeadState()
	if err != nil {
		headData, _ := os.ReadFile(".kitcat/HEAD")
		ref := strings.TrimSpace(string(headData))
		headState = strings.TrimPrefix(ref, "ref: refs/heads/")
	}
	fmt.Printf("[%s %s] %s\n%s\n", headState, newCommit.ID[:7], newCommit.Message, summary)
}

func main() {
	if len(os.Args) >= 4 && os.Args[1] == "branch" &&
		(os.Args[2] == "-m" || os.Args[2] == "--move") {
		newName := os.Args[3]
		err := core.RenameCurrentBranch(newName)
		if err != nil {
			fmt.Println("Error renaming branch:", err)
			os.Exit(1)
		}
		fmt.Println("Branch renamed to", newName)
		os.Exit(0)
	}
	if len(os.Args) < 2 {
		fmt.Println("Usage: kitcat <command> [args]")
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	if handler, ok := commands[cmd]; ok {
		handler(args)
	} else {
		fmt.Println("Unknown command:", cmd)
		core.PrintGeneralHelp()
		os.Exit(2)
	}
}
