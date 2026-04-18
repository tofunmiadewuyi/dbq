// Package main is the entry point for the dbq CLI.
package main

import (
	"fmt"
	"os"
	"strconv"
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
		id, lines, ok := parseLogsArgs(os.Args[2:])
		if !ok {
			fmt.Println("Usage: dbq logs <job-id> [--lines <N>]")
			return
		}
		printLogs(id, lines)

	case "config":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq config <job-id>")
			return
		}
		printConfig(os.Args[2])

	case "delete":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq delete <job-id>")
			return
		}
		deleteJob(os.Args[2])

	case "upgrade":
		upgrade()

	case "version", "-v":
		fmt.Println(version)

	case "help", "-h":
		fmt.Println("Usage: dbq <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  start            Open the interactive job manager")
		fmt.Println("  run <job-id>     Run a backup job by ID")
		fmt.Println("  logs <job-id> [--lines <N>]    Print the log history for a job (last N entries)")
		fmt.Println("  config <job-id>  Print the config file path and contents for a job")
		fmt.Println("  delete <job-id>  Delete a job by ID")
		fmt.Println("  upgrade          Upgrade dbq to the latest release")
		fmt.Println("  version          Print the current version")
		fmt.Println("  help             Show this help message")

	default:
		fmt.Println("Unknown command:", os.Args[1])
		fmt.Println("Run 'dbq help' for usage.")
	}

}

func parseLogsArgs(args []string) (id string, lines int, ok bool) {
	switch len(args) {
	case 1:
		return args[0], 0, true
	case 3:
		if args[1] != "--lines" {
			return "", 0, false
		}
		n, err := strconv.Atoi(args[2])
		if err != nil || n <= 0 {
			return "", 0, false
		}
		return args[0], n, true
	default:
		return "", 0, false
	}
}
