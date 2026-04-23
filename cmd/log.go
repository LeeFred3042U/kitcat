package main

import (
	"flag"
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleLog(args []string) {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	oneline := fs.Bool("oneline", false, "show compact log")

	_ = fs.Parse(args)

	commits, err := core.Log()
	if err != nil {
		die("log failed: %v", err)
	}

	for _, c := range commits {
		if *oneline {
			short := c.ID
			if len(short) > 7 {
				short = short[:7]
			}
			fmt.Printf("%s %s\n", short, c.Message)
			continue
		}

		fmt.Printf("commit %s\n", c.ID)
		fmt.Printf("Author: %s <%s>\n", c.AuthorName, c.AuthorEmail)
		fmt.Printf("Date:   %s\n\n", c.Timestamp.Format("Mon Jan 2 15:04:05 2006 -0700"))
		fmt.Printf("    %s\n\n", c.Message)
	}
}
