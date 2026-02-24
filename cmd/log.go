package main

import (
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

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
