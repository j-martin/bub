package main

import (
	"bufio"
	"fmt"
	"github.com/manifoldco/promptui"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

func AskForConfirmation(s string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", s)

		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}

func GetEnvWithDefault(key string, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		val = defaultValue
	}
	return val
}

func PathExists(fpath ...string) (bool, error) {
	_, err := os.Stat(path.Join(fpath...))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func EditFile(filePath string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	cmd := exec.Command(editor, filePath)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func JoinStringPointers(ptrs []*string, joinStr string) string {
	var arr []string
	for _, ref := range ptrs {
		if ref == nil {
			arr = append(arr, "")
		} else {
			arr = append(arr, *ref)
		}
	}
	return strings.Join(arr, joinStr)
}

func PickItem(label string, items []string) (string, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}:",
		Active:   "▶ {{ .}}",
		Inactive: "  {{ .}}",
		Selected: "▶ {{ .}}",
	}

	searcher := func(input string, index int) bool {
		i := items[index]
		name := strings.Replace(strings.ToLower(i), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Size:      20,
		Label:     label,
		Items:     items,
		Templates: templates,
		Searcher:  searcher,
	}
	i, _, err := prompt.Run()
	return items[i], err
}

func Random(min, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}

func HasNonEmptyLines(lines []string) bool {
	for _, s := range lines {
		if s != "" {
			return true
		}
	}
	return false
}

func ConditionalOp(message string, noop bool, fn func() error) error {
	if noop {
		log.Printf("%v (noop)", message)
		return nil
	}
	log.Printf(message)
	return fn()
}

func (g *Git) Update() {
	MustRunCmd("git", "clean", "-fd")
	MustRunCmd("git", "reset", "--hard")
	MustRunCmd("git", "checkout", "master", "-f")
	MustRunCmd("git", "pull")
	MustRunCmd("git", "pull", "--tags")
}

func (g *Git) ContainedUncommittedChanges() bool {
	return HasNonEmptyLines(strings.Split(MustRunCmdWithOutput("git", "status", "--short"), "\n"))
}

func (g *Git) IsDifferentFromMaster() bool {
	return HasNonEmptyLines(g.LogNotInMasterSubjects())
}
