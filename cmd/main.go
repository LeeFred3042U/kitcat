package main

import (
	"fmt"
	"os"
)

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

var commands = map[string]func([]string){
	"init":        handleInit,
	"add":         handleAdd,
	"commit":      handleCommit,
	"status":      handleStatus,
	"log":         handleLog,
	"branch":      handleBranch,
	"checkout":    handleCheckout,
	"clean":       handleClean,
	"rebase":      handleRebase,
	"merge":       handleMerge,
	"reset":       handleReset,
	"tag":         handleTag,
	"link":        handleLink,
	"diff":        handleDiff,
	"rm":          handleRm,
	"mv":          handleMv,
	"ls-files":    handleLsFiles,
	"show-object": handleShowObject,
	"config":      handleConfig,
	"help":        handleHelp,
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(exitUsage)
	}

	cmdName := os.Args[1]

	if cmdName == "-h" || cmdName == "--help" || cmdName == "help" {
		printHelp()
		os.Exit(exitSuccess)
	}

	handler, exists := commands[cmdName]
	if !exists {
		fmt.Fprintf(os.Stderr, "kitcat: '%s' is not a kitcat command. See 'kitcat --help'.\n", cmdName)
		os.Exit(exitFailure)
	}

	handler(os.Args[2:])
}

func handleHelp(args []string) {
	printHelp()
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "fatal: "+format+"\n", args...)
	os.Exit(exitFailure)
}

func printHelp() {
	helpText := `usage: kitcat <command> [<args>]

These are common kitcat commands used in various situations:

start a working area
   init       Initialize a new repository

work on the current change
   add        Add file contents to the index
   mv         Move or rename a file, a directory, or a symlink
   rm         Remove files from the working tree and from the index
   clean      Remove untracked files from the working tree

examine the history and state
   status     Show the working tree status
   log        Show commit logs
   diff       Show changes between commits, commit and working tree, etc
   ls-files   Show information about files in the index and the working tree
   show-object Display the contents of a kitcat object

grow, mark and tweak your common history
   branch     List, create, or delete branches
   checkout   Switch branches or restore working tree files
   commit     Record changes to the repository
   merge      Join two or more development histories together
   rebase     Reapply commits on top of another base tip
   reset      Reset current HEAD to the specified state
   tag        Create, list, delete or verify a tag object signed with GPG

configuration
   config     Get and set repository or global options
`
	fmt.Print(helpText)
}
