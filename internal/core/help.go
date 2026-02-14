package core

import "fmt"

// CommandHelp holds summary and usage text for a CLI command.
// Usage strings are printed verbatim to stdout.
type CommandHelp struct {
	Summary string
	Usage   string
}

// helpMessages maps command names to their help metadata.
// Stored as a map for lookup convenience; iteration order is not stable.
var helpMessages = map[string]CommandHelp{
	"init": {
		Summary: "Initialize a new KitKat repository",
		Usage:   "Usage: kitkat init\n\nInitializes a new .kitkat directory in the current folder, preparing it for tracking files",
	},
	"add": {
		Summary: "Add file contents to the index.",
		Usage:   "Usage: kitkat add <file-path> | --all | -A\n\nThis command adds file contents to the staging area.\nUse '--all' or '-A' to stage all new, modified, and deleted files.",
	},
	"commit": {
		Summary: "Record changes to the repository.",
		Usage:   "Usage: kitkat commit <-m | -am | --amend> <message>\n\nCreates a new commit from the staging area.\nUse '-am' to automatically stage all tracked files before committing.\nUse '--amend' to modify the previous commit.",
	},
	"diff": {
		Summary: "Show changes between the last commit and staging area",
		Usage:   "Usage: kitkat diff\n\nShows content differences between the HEAD commit and the index",
	},
	"log": {
		Summary: "Show the commit history",
		Usage:   "Usage: kitkat log [--oneline] [-n <limit>]\n\nDisplays the commit history for the current branch.\nFlags:\n  --oneline   Compact, single-line view\n  -n <limit>  Limits output to N commits",
	},
	"tag": {
		Summary: "Create a new tag for a commit",
		Usage:   "Usage: kitkat tag <tag-name> <commit-id>\n\nCreates a new lightweight tag that points to the specified commit",
	},
	"merge": {
		Summary: "Merge a branch into the current branch.",
		Usage:   "Usage: kitkat merge <branch-name>\n\nJoins another branch's history into the current branch. Currently, only fast-forward merges are supported.",
	},
	"ls-files": {
		Summary: "Show information about files in the index",
		Usage:   "Usage: kitkat ls-files\n\nPrints a list of all files that are currently in the index (staging area)",
	},
	"clean": {
		Summary: "Remove untracked files from the working directory",
		Usage:   "Usage: kitkat clean [-f] [-x]\n\nRemoves untracked files.\nFlags:\n  -f  Force deletion (required)\n  -x  Also delete ignored files",
	},
	"config": {
		Summary: "Get and set repository or global options.",
		Usage:   "Usage: kitkat config --global <key> <value>\n\nSets a global configuration value that will be used for all repositories.",
	},
	"reset": {
		Summary: "Reset current HEAD to the specified state",
		Usage:   "Usage: kitkat reset --hard <commit>\n\nResets the index and working tree. Any changes to tracked files in the working tree since <commit> are discarded.",
	},
	"checkout": {
		Summary: "Switch branches or restore working tree files",
		Usage:   "Usage: kitkat checkout <branch> or checkout -b <new-branch>\n\nSwitches to a branch. Use -b to create a new branch and switch to it.",
	},
	"show-object": {
		Summary: "Provide content or type and size information for repository objects",
		Usage:   "Usage: kitkat show-object <hash>\n\nShows the contents of the object identified by the hash.",
	},
	"branch": {
		Summary: "List, create, or delete branches",
		Usage:   "Usage: kitkat branch <name> or branch -m <new-name>\n\nCreates a new branch. Use -m to rename an existing branch.",
	},
	"mv": {
		Summary: "Move or rename a file, a directory, or a symlink",
		Usage:   "Usage: kitkat mv <old> <new>\n\nRenames the file/directory <old> to <new>.",
	},
}

// PrintGeneralHelp lists all available commands with short summaries.
// Output order depends on map iteration and is not guaranteed to be stable.
func PrintGeneralHelp() {
	fmt.Println("usage: kitkat <command> [arguments]")
	fmt.Println("\nThese are the common KitKat commands:")
	for name, help := range helpMessages {
		fmt.Printf("   %-12s %s\n", name, help.Summary)
	}
	fmt.Println("\nUse 'kitkat help <command>' for more information about a command")
}

// PrintCommandHelp prints detailed usage text for a specific command.
// Unknown commands produce a fallback message.
func PrintCommandHelp(command string) {
	if help, ok := helpMessages[command]; ok {
		fmt.Println(help.Usage)
	} else {
		fmt.Printf("Unknown help topic: '%s'. See 'kitkat --help'.\n", command)
	}
}
