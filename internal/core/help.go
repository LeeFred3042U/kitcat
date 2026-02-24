package core

import (
	"text/template"
	"strings"
	"sort"
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/constant"
)

// CommandHelp holds summary and usage text for a CLI command.
type CommandHelp struct {
	Summary string
	Usage   string
}

// helpMessages maps command names to help metadata.
// Uses Go templates for dynamic values.
var helpMessages = map[string]CommandHelp{
	"init": {
		Summary: "Initialize a new {{.APP}} repository",
		Usage:   "Usage: {{.APP}} init\n\nInitializes a new {{.DIR}} directory in the current folder, preparing it for tracking files",
	},
	"add": {
		Summary: "Add file contents to the index.",
		Usage:   "Usage: {{.APP}} add <file-path> | --all | -A\n\nThis command adds file contents to the staging area.\nUse '--all' or '-A' to stage all new, modified, and deleted files.",
	},
	"commit": {
		Summary: "Record changes to the repository.",
		Usage:   "Usage: {{.APP}} commit <-m | -am | --amend> <message>\n\nCreates a new commit from the staging area.\nUse '-am' to automatically stage all tracked files before committing.\nUse '--amend' to modify the previous commit.",
	},
	"diff": {
		Summary: "Show changes between the last commit and staging area",
		Usage:   "Usage: {{.APP}} diff\n\nShows content differences between the HEAD commit and the index",
	},
	"log": {
		Summary: "Show the commit history",
		Usage:   "Usage: {{.APP}} log [--oneline] [-n <limit>]\n\nDisplays the commit history for the current branch.\nFlags:\n  --oneline   Compact, single-line view\n  -n <limit>  Limits output to N commits",
	},
	"tag": {
		Summary: "Create a new tag for a commit",
		Usage:   "Usage: {{.APP}} tag <tag-name> <commit-id>\n\nCreates a new lightweight tag that points to the specified commit",
	},
	"merge": {
		Summary: "Merge a branch into the current branch.",
		Usage:   "Usage: {{.APP}} merge <branch-name>\n\nJoins another branch's history into the current branch. Currently, only fast-forward merges are supported.",
	},
	"ls-files": {
		Summary: "Show information about files in the index",
		Usage:   "Usage: {{.APP}} ls-files\n\nPrints a list of all files that are currently in the index (staging area)",
	},
	"clean": {
		Summary: "Remove untracked files from the working directory",
		Usage:   "Usage: {{.APP}} clean [-f] [-x]\n\nRemoves untracked files.\nFlags:\n  -f  Force deletion (required)\n  -x  Also delete ignored files",
	},
	"config": {
		Summary: "Get and set repository or global options.",
		Usage:   "Usage: {{.APP}} config --global <key> <value>\n\nSets a global configuration value that will be used for all repositories.",
	},
	"reset": {
		Summary: "Reset current HEAD to the specified state",
		Usage:   "Usage: {{.APP}} reset --hard <commit>\n\nResets the index and working tree. Any changes to tracked files in the working tree since <commit> are discarded.",
	},
	"checkout": {
		Summary: "Switch branches or restore working tree files",
		Usage:   "Usage: {{.APP}} checkout <branch> or checkout -b <new-branch>\n\nSwitches to a branch. Use -b to create a new branch and switch to it.",
	},
	"show-object": {
		Summary: "Provide content or type and size information for repository objects",
		Usage:   "Usage: {{.APP}} show-object <hash>\n\nShows the contents of the object identified by the hash.",
	},
	"branch": {
		Summary: "List, create, or delete branches",
		Usage:   "Usage: {{.APP}} branch <name> or branch -m <new-name>\n\nCreates a new branch. Use -m to rename an existing branch.",
	},
	"mv": {
		Summary: "Move or rename a file, a directory, or a symlink",
		Usage:   "Usage: {{.APP}} mv <old> <new>\n\nRenames the file/directory <old> to <new>.",
	},
}

// templateData is passed into templates.
var templateData = map[string]string{
	"APP": constant.AppName,
	"DIR": constant.RepoDir,
}

// renderTemplate parses and prints a template string.
func renderTemplate(text string) string {
	tmpl := template.Must(template.New("").Parse(text))

	var out string
	buf := &strings.Builder{}
	if err := tmpl.Execute(buf, templateData); err == nil {
		out = buf.String()
	}
	return out
}

// PrintGeneralHelp lists all commands.
func PrintGeneralHelp() {
	fmt.Printf("usage: %s <command> [arguments]\n", constant.AppName)
	fmt.Printf("\nThese are the common %s commands:\n", constant.AppName)

	// Stable order (maps are random)
	var keys []string
	for k := range helpMessages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		help := helpMessages[name]
		summary := renderTemplate(help.Summary)
		fmt.Printf("   %-12s %s\n", name, summary)
	}

	fmt.Printf("\nUse '%s help <command>' for more information about a command\n", constant.AppName)
}

// PrintCommandHelp prints detailed usage.
func PrintCommandHelp(command string) {
	if help, ok := helpMessages[command]; ok {
		fmt.Println(renderTemplate(help.Usage))
		return
	}

	fmt.Printf("Unknown help topic: '%s'. See '%s --help'.\n", command, constant.AppName)
}
