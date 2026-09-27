// Package main is the entry point for the dbq CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/tofunmiadewuyi/dbq/internal/agent"
	"github.com/tofunmiadewuyi/dbq/internal/engine"
	"github.com/tofunmiadewuyi/dbq/internal/secrets"
)

type Session struct {
	secrets secrets.Provider
}

func newSession() *Session {
	provider, err := secrets.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not initialize secure secret storage: %v\n", err)
		os.Exit(1)
	}
	return &Session{secrets: provider}
}

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Usage: dbq <cmd>")
		return
	}

	switch os.Args[1] {
	case "agent":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: dbq agent")
			os.Exit(2)
		}
		os.Exit(agent.Run(context.Background(), os.Stdin, os.Stdout, os.Stderr, os.Getenv, engine.New()))

	case "start":
		newSession().startCLI()

	case "run":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq run <job>")
			return
		}
		newSession().runJob(os.Args[2])

	case "logs":
		id, lines, ok := parseLogsArgs(os.Args[2:])
		if !ok {
			fmt.Println("Usage: dbq logs <job-id> [--lines <N>]")
			return
		}
		(&Session{}).printLogs(id, lines)

	case "delete":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq delete <job-id>")
			return
		}
		newSession().deleteJob(os.Args[2])

	case "config":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq config <job-id>")
			return
		}
		printConfig(os.Args[2])

	case "prune":
		if len(os.Args) != 3 {
			fmt.Println("Usage: dbq prune <job-id>")
			return
		}
		newSession().pruneJob(os.Args[2])

	case "upgrade":
		upgrade()

	case "uninstall":
		uninstall()

	case "version", "-v":
		fmt.Println(version)

	case "help", "-h":
		fmt.Println("Usage: dbq <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  agent           Execute one JSON request from stdin and write one JSON result")
		fmt.Println("  start            Open the interactive job manager")
		fmt.Println("  run <job-id>     Run a backup job by ID")
		fmt.Println("  logs <job-id> [--lines <N>]    Print the log history for a job (last N entries)")
		fmt.Println("  config <job-id>  Print the config file path and contents for a job")
		fmt.Println("  prune <job-id>   Delete old backups now, per the job's retention setting")
		fmt.Println("  delete <job-id>  Delete a job by ID")
		fmt.Println("  upgrade          Upgrade dbq to the latest release")
		fmt.Println("  uninstall        Remove the dbq binary")
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
