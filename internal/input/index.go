// Package input handles user prompts, validation, and selection menus.
package input

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func Ask(question string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(question)
	answer, _ := reader.ReadString('\n')
	return strings.TrimSpace(answer)
}

func AskWithDefault(question, defaultVal string) string {
	fmt.Printf("%s [%s]: ", question, defaultVal)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return defaultVal
	}
	return answer
}

func AskValid(question string, validate func(string) error, def string) string {
	for {
		var answer string
		if def == "" {
			answer = Ask(question)
		} else {
			answer = AskWithDefault(question, def)
		}
		if err := validate(answer); err != nil {
			fmt.Println("invalid:", err)
			continue
		}
		return answer
	}

}

func AskValidInt(q string, validate func(string) error, def string) int {
	ans := AskValid(q, validate, def)
	i, err := strconv.Atoi(ans)
	if err != nil {
		fmt.Println("invalid:", err)
		AskValidInt(q, validate, def)
	}

	return i

}

type Option struct {
	Label  string
	Action func() error
}

// ChooseAndExec allows you ask a question and provide options
// Calls the Action on the selected Option
func ChooseAndExec(question string, options []Option) error {
	fmt.Println(question)
	for i, opt := range options {
		fmt.Printf("%d) %s\n", i+1, opt.Label)
	}
	answer := Ask("select: ")
	i, err := strconv.Atoi(answer)
	if err != nil || i < 1 || i > len(options) {
		fmt.Println("invalid selection")
		return ChooseAndExec(question, options)
	}
	return options[i-1].Action()
}

func Choose(question string, options []string) string {
	fmt.Println(question)
	for i, opt := range options {
		fmt.Printf("%d) %s\n", i+1, opt)
	}
	answer := Ask("select: ")
	i, err := strconv.Atoi(answer)
	if err != nil || i < 1 || i > len(options) {
		fmt.Println("invalid selection")
		return Choose(question, options)
	}
	return options[i-1]
}

// Todo is a default Action for Option
// Remove all Todos in release
func Todo(name string) func() error {
	return func() error {
		fmt.Printf("%s: not implemented\n", name)
		return nil
	}
}
