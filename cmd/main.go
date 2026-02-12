package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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

// --- Handlers ---

func handleInit(args []string) {
	if err := core.Init(); err != nil {
		die("init failed: %v", err)
	}
	fmt.Fprintln(os.Stderr, "Initialized empty kitcat repository")
}

func handleAdd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat add <pathspec>...")
		os.Exit(exitUsage)
	}

	if len(args) == 1 && (args[0] == "." || args[0] == "-A" || args[0] == "--all") {
		if err := core.AddAll(); err != nil {
			die("add all failed: %v", err)
		}
		return
	}

	hasError := false
	for _, path := range args {
		if err := core.AddFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "error: adding '%s' failed: %v\n", path, err)
			hasError = true
		}
	}

	if hasError {
		os.Exit(exitFailure)
	}
}

func handleCommit(args []string) {
	var cleanArgs []string
	for _, a := range args {
		if strings.HasPrefix(a, "-am=") {
			cleanArgs = append(cleanArgs, "-a", "-m", strings.TrimPrefix(a, "-am="))
			continue
		}
		if strings.HasPrefix(a, "-am") && a != "-a" && a != "-m" {
			cleanArgs = append(cleanArgs, "-a", "-m")
			if len(a) > 3 {
				cleanArgs = append(cleanArgs, a[3:])
			}
			continue
		}
		cleanArgs = append(cleanArgs, a)
	}

	fs := flag.NewFlagSet("commit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	msg := fs.String("m", "", "Commit message")
	amend := fs.Bool("amend", false, "Amend the last commit")
	all := fs.Bool("a", false, "Stage all modified/deleted files")

	if err := fs.Parse(cleanArgs); err != nil {
		os.Exit(exitUsage)
	}

	// Architectural fix: Use core.GetHeadCommit instead of core.Log
	if *amend && *msg == "" {
		if head, err := core.GetHeadCommit(); err == nil {
			*msg = head.Message
		}
	}

	if *msg == "" {
		fmt.Fprintln(os.Stderr, "error: commit message required (use -m)")
		os.Exit(exitUsage)
	}

	var hash string
	var err error

	if *amend {
		hash, err = core.AmendCommit(*msg)
	} else if *all {
		hash, err = core.CommitAll(*msg)
	} else {
		hash, err = core.Commit(*msg)
	}

	if err != nil {
		if err.Error() == "nothing to commit, working tree clean" {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(exitFailure)
		}
		die("commit failed: %v", err)
	}

	if len(hash) >= 7 {
		fmt.Printf("[%s] %s\n", hash[:7], *msg)
	} else {
		fmt.Printf("[%s] %s\n", hash, *msg)
	}
}

func handleStatus(args []string) {
	out, err := core.Status()
	if err != nil {
		die("status failed: %v", err)
	}
	fmt.Println(out)
}

func handleLog(args []string) {
	commits, err := core.Log()
	if err != nil {
		die("log failed: %v", err)
	}

	for _, c := range commits {
		fmt.Printf("commit %s\n", c.ID)
		fmt.Printf("Author: %s <%s>\n", c.AuthorName, c.AuthorEmail)
		fmt.Printf("Date:   %s\n\n", c.Timestamp.Format("Mon Jan 2 15:04:05 2006 -0700"))
		fmt.Printf("    %s\n\n", c.Message)
	}
}

func handleBranch(args []string) {
	if len(args) == 0 {
		if err := core.ListBranches(); err != nil {
			die("%v", err)
		}
		return
	}

	if args[0] == "-d" || args[0] == "--delete" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: kitcat branch -d <branchname>")
			os.Exit(exitUsage)
		}
		if err := core.DeleteBranch(args[1]); err != nil {
			die("%v", err)
		}
		fmt.Fprintf(os.Stderr, "Deleted branch %s\n", args[1])
		return
	}

	if args[0] == "-m" || args[0] == "--move" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: kitcat branch -m <newname>")
			os.Exit(exitUsage)
		}
		if err := core.RenameCurrentBranch(args[1]); err != nil {
			die("%v", err)
		}
		fmt.Fprintf(os.Stderr, "Renamed current branch to %s\n", args[1])
		return
	}

	if err := core.CreateBranch(args[0]); err != nil {
		die("%v", err)
	}
	fmt.Fprintf(os.Stderr, "Created branch %s\n", args[0])
}

func handleCheckout(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat checkout <branch> | -b <new_branch>")
		os.Exit(exitUsage)
	}

	if args[0] == "-b" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: kitcat checkout -b <new_branch>")
			os.Exit(exitUsage)
		}
		newBranch := args[1]
		if err := core.CreateBranch(newBranch); err != nil {
			die("failed to create branch: %v", err)
		}
		if err := core.Checkout(newBranch); err != nil {
			die("failed to checkout new branch: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Switched to a new branch '%s'\n", newBranch)
		return
	}

	arg := args[0]
	if err := core.Checkout(arg); err != nil {
		die("%v", err)
	}

	fmt.Fprintf(os.Stderr, "Checked out '%s'\n", arg)
}

func handleClean(args []string) {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dryRun := fs.Bool("n", false, "Dry run")
	force := fs.Bool("f", false, "Force")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if !*force && !*dryRun {
		fmt.Fprintln(os.Stderr, "fatal: clean.requireForce defaults to true and neither -n nor -f given; refusing to clean")
		os.Exit(exitFailure)
	}

	if err := core.Clean(*dryRun); err != nil {
		die("%v", err)
	}
}

func handleRebase(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat rebase <branch> [-i | --abort | --continue]")
		os.Exit(exitUsage)
	}

	arg := args[0]

	if arg == "--abort" {
		if err := core.RebaseAbort(); err != nil {
			die("%v", err)
		}
		return
	}
	if arg == "--continue" {
		if err := core.RebaseContinue(); err != nil {
			die("%v", err)
		}
		return
	}

	interactive := false
	target := arg

	if len(args) > 1 && (args[1] == "-i" || args[1] == "--interactive") {
		interactive = true
	}

	if err := core.Rebase(target, interactive); err != nil {
		die("%v", err)
	}
}

func handleMerge(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat merge <branch>")
		os.Exit(exitUsage)
	}
	if err := core.Merge(args[0]); err != nil {
		die("%v", err)
	}
}

func handleReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	hard := fs.Bool("hard", false, "Reset index and working tree")
	soft := fs.Bool("soft", false, "Reset only HEAD")
	_ = fs.Bool("mixed", false, "Reset HEAD and index (default)")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if *hard && *soft {
		fmt.Fprintln(os.Stderr, "fatal: --hard and --soft are mutually exclusive")
		os.Exit(exitUsage)
	}

	fsArgs := fs.Args()
	commit := "HEAD"
	if len(fsArgs) > 0 {
		commit = fsArgs[0]
	}

	mode := core.ResetMixed
	if *hard {
		mode = core.ResetHard
	} else if *soft {
		mode = core.ResetSoft
	}

	if err := core.Reset(commit, mode); err != nil {
		die("%v", err)
	}
}

func handleTag(args []string) {
	if len(args) == 0 {
		if err := core.PrintTags(); err != nil {
			die("%v", err)
		}
		return
	}

	tagName := args[0]
	commit := "HEAD"
	if len(args) > 1 {
		commit = args[1]
	}

	// Architectural fix: Use core.ResolveHead
	if commit == "HEAD" {
		headHash, err := core.ResolveHead()
		if err != nil {
			die("cannot resolve HEAD: %v", err)
		}
		if headHash == "" {
			die("cannot resolve HEAD: ref is empty or invalid")
		}
		commit = headHash
	}

	if err := core.CreateTag(tagName, commit); err != nil {
		die("%v", err)
	}
}

func handleDiff(args []string) {
	// Check for --staged flag
	staged := false
	if len(args) > 0 && (args[0] == "--staged" || args[0] == "--cached") {
		staged = true
	}
	if err := core.Diff(staged); err != nil { // Pass arg
		die("%v", err)
	}
}

func handleRm(args []string) {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	recursive := fs.Bool("r", false, "Allow recursive removal")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat rm [-r] <file>...")
		os.Exit(exitUsage)
	}

	for _, file := range fs.Args() {
		if err := core.RemoveFile(file, *recursive); err != nil {
			die("%v", err)
		}
	}
}

func handleMv(args []string) {
	fs := flag.NewFlagSet("mv", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	force := fs.Bool("f", false, "Force move/rename")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat mv <source> <destination>")
		os.Exit(exitUsage)
	}

	if err := core.MoveFile(fs.Arg(0), fs.Arg(1), *force); err != nil {
		die("%v", err)
	}
	fmt.Fprintf(os.Stderr, "Renamed '%s' to '%s'\n", fs.Arg(0), fs.Arg(1))
}

func handleLsFiles(args []string) {
	if err := core.ListFiles(); err != nil {
		die("%v", err)
	}
}

func handleShowObject(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat show-object <hash>")
		os.Exit(exitUsage)
	}
	if err := core.ShowObject(args[0]); err != nil {
		die("%v", err)
	}
}

func handleConfig(args []string) {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var global bool
	fs.BoolVar(&global, "global", false, "Use global config file")
	fs.BoolVar(&global, "g", false, "Use global config file")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	params := fs.Args()

	if len(params) < 1 {
		if err := core.PrintAllConfig(); err != nil {
			die("%v", err)
		}
		return
	}

	key := params[0]
	if len(params) == 2 {
		val := params[1]
		if err := core.SetConfig(key, val, global); err != nil {
			die("%v", err)
		}
		return
	}

	val, found, err := core.GetConfig(key)
	if err != nil {
		die("%v", err)
	}
	if found {
		fmt.Println(val)
	}
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
