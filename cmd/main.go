// Package main is the entry point for the dbq CLI.
package main

import (
	"fmt"
	"os"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Usage: dbq <cmd>")
		return
	}

	switch os.Args[1] {
	case "start":
		startCLI()

	case "run":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq run <job>")
			return
		}
		runJob(os.Args[2])

	case "upgrade":
		upgrade()

	case "version":
		fmt.Println(version)

	default:
		fmt.Println("Unknown command:", os.Args[1])
	}

}
