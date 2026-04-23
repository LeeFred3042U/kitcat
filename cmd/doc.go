// Package cmd implements the command-line interface for kitcat.
//
// # Command Design
//
// Commands in this package follow the same high-level structure used in tools
// like Git and Docker: each top-level command defines how it interprets its
// arguments based on either subcommands or flags.
//
// The choice between these patterns is intentional and should remain consistent
// across the codebase.
//
// _____________________________________________________________________________
//
// # Subcommand-style Commands
//
// Commands that expose multiple distinct operations use subcommands. The first
// non-flag argument selects the execution path.
//
// Example:
//
//	kitcat stash apply
//	kitcat stash drop
//	kitcat stash pop
//
// Pattern:
//
//	sub := args[0]
//	switch sub {
//	case "apply":
//	case "drop":
//	}
//
// Used in:
//
//	handleStash
//
// These commands behave like a dispatcher. Each subcommand represents a
// different operation and should be handled explicitly.
//
// _____________________________________________________________________________
//
// # Flag-driven Commands
//
// Commands whose behavior is modified by options use flags. Flags are parsed
// using flag.FlagSet, and the remaining arguments are treated as positional
// inputs.
//
// Example:
//
//	kitcat commit -m "msg"
//	kitcat branch -v
//	kitcat tag -d v1.0
//
// Pattern:
//
//	fs := flag.NewFlagSet("cmd", flag.ExitOnError)
//	opt := fs.Bool("flag", false, "description")
//	fs.Parse(args)
//
//	if *opt {
//	    // modify behavior
//	}
//
// Used in:
//
//	handleCommit
//	handleBranch
//	handleTag
//	handleCheckout
//	handleRebase
//	handleAdd
//	handleClean
//	handleReset
//	handleMv
//
// These commands represent a single operation whose behavior is altered by
// flags rather than split into separate subcommands.
//
// _____________________________________________________________________________
//
// # Hybrid Commands
//
// Some commands combine flag-based parsing with control-flow flags that alter
// execution paths.
//
// Example:
//
//	kitcat rebase --abort
//	kitcat rebase --continue
//	kitcat rebase -i main
//
// Pattern:
//
//	if *abort {
//	    ...
//	} else if *continue {
//	    ...
//	} else {
//	    ...
//	}
//
// Used in:
//
//	handleRebase
//
// _____________________________________________________________________________
//
// # Argument Handling
//
// After calling:
//
//	fs.Parse(args)
//
// All positional arguments must be read from:
//
//	fs.Args()
//
// The original args slice must not be used after parsing, as it still contains
// flag tokens.
//
// _____________________________________________________________________________
//
// # Design Guidelines
//
//   - Use subcommands when the command represents multiple distinct actions.
//   - Use flags when modifying the behavior of a single action.
//   - Follow existing patterns in this package when adding new commands.
//   - Prefer consistency with Git-style CLI semantics.
//
// This approach keeps the CLI predictable, composable, and aligned with
// established command-line conventions.
//
package main
