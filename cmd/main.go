package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
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
		core.PrintGeneralHelp()
		os.Exit(exitUsage)
	}

	cmdName := os.Args[1]

	if cmdName == "-h" || cmdName == "--help" || cmdName == "help" {
		if len(os.Args) > 2 {
			core.PrintCommandHelp(os.Args[2])
		} else {
			core.PrintGeneralHelp()
		}
		os.Exit(exitSuccess)
	}

	handler, exists := commands[cmdName]
	if !exists {
		fmt.Fprintf(os.Stderr, "%s: '%s' is not a %s command. See '%s --help'.\n", app.Name, cmdName, app.Name, app.Name)
		os.Exit(exitFailure)
	}

	handler(os.Args[2:])
}

func handleHelp(args []string) {
	if len(args) > 0 {
		core.PrintCommandHelp(args[0])
	} else {
		core.PrintGeneralHelp()
	}
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "fatal: "+format+"\n", args...)
	os.Exit(exitFailure)
}
