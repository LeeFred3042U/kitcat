package core

import "fmt"

type CommandHelp struct {
	Summary string
	Usage   string
}

var helpMessages = map[string]CommandHelp{
	"init": {
		Summary: "Initialize a new KitCat repository",
		Usage:   "Usage: kitcat init\n\nInitializes a new .kitcat directory in the current folder, preparing it for tracking files",
	},
	"add": {
		Summary: "Add file contents to the index.",
		Usage:   "Usage: kitcat add <file-path> | --all | -A\n\nThis command adds file contents to the staging area.\nUse '--all' or '-A' to stage all new, modified, and deleted files.",
	},
	"commit": {
		Summary: "Record changes to the repository.",
		Usage:   "Usage: kitcat commit <-m | -am | --amend> <message>\n\nCreates a new commit from the staging area.\nUse '-am' to automatically stage all tracked files before committing.\nUse '--amend' to modify the previous commit.",
	},
	"diff": {
		Summary: "Show changes between the last commit and staging area",
		Usage:   "Usage: kitcat diff\n\nShows content differences between the HEAD commit and the index",
	},
	"log": {
		Summary: "Show the commit history",
		Usage:   "Usage: kitcat log [--oneline] [-n <limit>]\n\nDisplays the commit history for the current branch.\nFlags:\n  --oneline   Compact, single-line view\n  -n <limit>  Limits output to N commits",
	},
	"tag": {
		Summary: "Create a new tag for a commit",
		Usage:   "Usage: kitcat tag <tag-name> <commit-id>\n\nCreates a new lightweight tag that points to the specified commit",
	},
	"merge": {
		Summary: "Merge a branch into the current branch.",
		Usage:   "Usage: kitcat merge <branch-name>\n\nJoins another branch's history into the current branch. Currently, only fast-forward merges are supported.",
	},
	"ls-files": {
		Summary: "Show information about files in the index",
		Usage:   "Usage: kitcat ls-files\n\nPrints a list of all files that are currently in the index (staging area)",
	},
	"clean": {
		Summary: "Remove untracked files from the working directory",
		Usage:   "Usage: kitcat clean [-f] [-x]\n\nRemoves untracked files.\nFlags:\n  -f  Force deletion (required)\n  -x  Also delete ignored files",
	},
	"config": {
		Summary: "Get and set repository or global options.",
		Usage:   "Usage: kitcat config --global <key> <value>\n\nSets a global configuration value that will be used for all repositories.",
	},
	"reset": {
		Summary: "Reset current HEAD to the specified state",
		Usage:   "Usage: kitcat reset --hard <commit>\n\nResets the index and working tree. Any changes to tracked files in the working tree since <commit> are discarded.",
	},
	"checkout": {
		Summary: "Switch branches or restore working tree files",
		Usage:   "Usage: kitcat checkout <branch> or checkout -b <new-branch>\n\nSwitches to a branch. Use -b to create a new branch and switch to it.",
	},
	"show-object": {
		Summary: "Provide content or type and size information for repository objects",
		Usage:   "Usage: kitcat show-object <hash>\n\nShows the contents of the object identified by the hash.",
	},
	"branch": {
		Summary: "List, create, or delete branches",
		Usage:   "Usage: kitcat branch <name> or branch -m <new-name>\n\nCreates a new branch. Use -m to rename an existing branch.",
	},
	"mv": {
		Summary: "Move or rename a file, a directory, or a symlink",
		Usage:   "Usage: kitcat mv <old> <new>\n\nRenames the file/directory <old> to <new>.",
	},
	"status": {
		Summary: "Show the working tree status",
		Usage:   "Usage: kitcat status\n\nDisplays paths that have differences between the working tree, the index and the last commit. Shows staged, unstaged and untracked files.",
	},
	"stash": {
		Summary: "Stash the current working directory changes",
		Usage:   "Usage: kitcat stash\n\nTemporarily saves changes in the working directory and index, allowing you to work on a clean state and reapply them later.",
	},
	"rebase": {
		Summary: "Reapply commits on top of another base commit",
		Usage:   "Usage: kitcat rebase <branch>\n\nReapplies the current branch commits on top of the specified branch, resulting in a linear commit history.",
	},
	"grep": {
		Summary: "Search for patterns in tracked files",
		Usage:   "Usage: kitcat grep <pattern>\n\nSearches through tracked files in the repository and prints lines matching the given pattern.",
	},
	"shortlog": {
		Summary: "Summarize commit history by author",
		Usage:   "Usage: kitcat shortlog\n\nDisplays a condensed summary of commit history, grouped by author, showing commit counts and messages.",
	},
	"rm": {
		Summary: "Remove files from the working tree and index",
		Usage:   "Usage: kitcat rm <file-path>\n\nRemoves the specified file from the working directory & stages the removal for the next commit.",
	},
}

func PrintGeneralHelp() {
	fmt.Println("usage: kitcat <command> [arguments]")
	fmt.Println("\nThese are the common KitCat commands:")
	for name, help := range helpMessages {
		fmt.Printf("   %-12s %s\n", name, help.Summary)
	}
	fmt.Println("\nUse 'kitcat help <command>' for more information about a command")
}

func PrintCommandHelp(command string) {
	if help, ok := helpMessages[command]; ok {
		fmt.Println(help.Usage)
	} else {
		fmt.Printf("Unknown help topic: '%s'. See 'kitcat --help'.\n", command)
	}
}
