package main

import (
	"flag"
	"fmt"
)

func addQuietFlag(fs *flag.FlagSet) *bool {
	q := fs.Bool("q", false, "suppress output")
	fs.BoolVar(q, "quiet", false, "suppress output")
	return q
}

func printIfNotQuiet(q bool, format string, a ...any) {
	if q {
		return
	}
	fmt.Printf(format, a...)
}

func shortHash(h string) string {
	if len(h) >= 7 {
		return h[:7]
	}
	return h
}
func parseStashIndex(arg string) int {
	var n int
	_, err := fmt.Sscanf(arg, "stash@{%d}", &n)
	if err != nil {
		die("invalid stash reference: %s", arg)
	}
	return n
}
