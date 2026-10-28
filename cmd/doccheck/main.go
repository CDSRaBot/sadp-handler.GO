package main

import (
	"fmt"
	"os"

	"github.com/CDSRaBot/sadp-handler.GO/internal/doccheck"
)

func main() {
	violations := doccheck.Check(".")
	if len(violations) == 0 {
		fmt.Println("documentation consistency check passed")
		return
	}
	for _, violation := range violations {
		fmt.Fprintln(os.Stderr, violation)
	}
	os.Exit(1)
}
