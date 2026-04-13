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

	case "logs":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq logs <job-id>")
			return
		}
		printLogs(os.Args[2])

	case "config":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq config <job-id>")
			return
		}
		printConfig(os.Args[2])

	case "upgrade":
		upgrade()

	case "version":
		fmt.Println(version)

	case "help":
		fmt.Println("Usage: dbq <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  start            Open the interactive job manager")
		fmt.Println("  run <job-id>     Run a backup job by ID")
		fmt.Println("  logs <job-id>    Print the log history for a job")
		fmt.Println("  config <job-id>  Print the config file path and contents for a job")
		fmt.Println("  upgrade          Upgrade dbq to the latest release")
		fmt.Println("  version          Print the current version")
		fmt.Println("  help             Show this help message")

	default:
		fmt.Println("Unknown command:", os.Args[1])
		fmt.Println("Run 'dbq help' for usage.")
	}

}
