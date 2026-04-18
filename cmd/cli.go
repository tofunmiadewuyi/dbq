package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tofunmiadewuyi/dbq/internal/input"
	"github.com/tofunmiadewuyi/dbq/internal/job"
	"github.com/tofunmiadewuyi/dbq/internal/workflow"
	"github.com/tofunmiadewuyi/dbq/utils"
)

func startCLI() {
	// at startup
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

		var b strings.Builder
		fmt.Fprintf(&b, "\n┌%s┐\n", border)
		fmt.Fprintf(&b, "│%s│\n", box.BoxCenter("WELCOME TO DBQ BY 7A"))
		fmt.Fprintf(&b, "├%s┤\n", border)
		b.WriteString(box.RowStr(" ", "What would you like to do?"))
		for i, opt := range menuOptions {
			b.WriteString(box.RowStr(fmt.Sprintf("%d)  ", i+1), opt.Label))
		}
		fmt.Fprintf(&b, "└%s┘\n\n", border)

		content := b.String()
		fmt.Print(content)

		selection := input.AskValidInt("Select: ", func(n string) error {
			return input.ValidateInt("A selection", n)
		}, "")

		utils.DimPrevious(content, selection)

		if err := menuOptions[selection-1].Action(); err != nil {
			fmt.Println("error:", err)
		}
	}

}

