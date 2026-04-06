// Package main is the entry point for the dbq CLI.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tofunmiadewuyi/dbq/utils"
	"github.com/tofunmiadewuyi/dbq/internal/config"
	"github.com/tofunmiadewuyi/dbq/internal/input"
	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/internal/workflow"
)

func cleanStaleTempFiles() {
	tmpDir := filepath.Join(config.TmpPath, config.AppName)
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if time.Since(info.ModTime()) > 24*time.Hour {
			os.Remove(path)
		}
		return nil
	})
}

func main() {
	// clean on startup
	cleanStaleTempFiles()
	for {
		jobs, err := job.GetJobs()
		if err != nil && !os.IsNotExist(err) {
			fmt.Println("error reading jobs:", err)
			os.Exit(1)
		}

		menuOptions := []input.Option{}
		if len(jobs) > 0 {
			menuOptions = append(menuOptions, input.Option{
				Label:  fmt.Sprintf("Manage Jobs (%d)", len(jobs)),
				Action: func() error { return workflow.ManageJobs(jobs) },
			})
		}
		menuOptions = append(menuOptions, input.Option{
			Label:  "New Job...",
			Action: job.StartNewJob,
		})
		menuOptions = append(menuOptions, input.Option{
			Label:  "Exit",
			Action: func() error { os.Exit(0); return nil },
		})

		w := 68
		box := utils.NewDisplayBox(w)
		border := box.BoxBorder()
		center := box.BoxCenter
		row := box.CreateRow

		fmt.Printf("\n┌%s┐\n", border)
		fmt.Printf("│%s│\n", center("WELCOME TO DBQ BY 7A"))
		fmt.Printf("├%s┤\n", border)

		row(" ", "What would you like to do?")
		for i, opt := range menuOptions {
			row(fmt.Sprintf("%d)  ", i+1), opt.Label)
		}

		fmt.Printf("└%s┘\n\n", border)

		selection := input.AskValidInt("Select: ", func(n string) error {
			return input.ValidateInt("A selection", n)
		}, "")

		if err := menuOptions[selection-1].Action(); err != nil {
			fmt.Println("error:", err)
		}
	}
}
