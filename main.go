package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/chzyer/readline"
	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"
)

var (
	resourceGroupToPassOn string
	version               string      = "v0.0.1"
	appAuthor             *cli.Author = &cli.Author{
		Name:  "LuciferInLove",
		Email: "lucifer.in.love@protonmail.com",
	}
)

const (
	sshExecutable = "ssh"
)

func main() {
	app := &cli.App{
		Name:            "dynamic-sshmenu-azure",
		Usage:           "builds dynamic azure vm addresses list like sshmenu",
		Authors:         []*cli.Author{appAuthor},
		Action:          action,
		UsageText:       "dynamic-sshmenu-azure [-- <args>]",
		Version:         version,
		HideHelpCommand: true,
	}

	app.Commands = []*cli.Command{}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "tags",
			Aliases: []string{"t"},
			Usage:   "instance tags in \"key1:value1;key2:value2\" format. If undefined, full list will be shown",
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "resource-group",
			Aliases: []string{"g"},
			Usage:   "azure resource group name. If undefined, resource groups list will be shown. Environment variables: ",
			EnvVars: []string{"AZURE_DEFAULTS_GROUP", "AZURE_BASE_GROUP_NAME"},
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "location",
			Aliases: []string{"l"},
			Usage:   "azure resource groups location (region). If undefined, full resource groups list will be shown. Environment variables: ",
			EnvVars: []string{"AZURE_DEFAULTS_LOCATION"},
			Value:   "",
		},
		&cli.BoolFlag{
			Name:    "public-ip",
			Aliases: []string{"p"},
			Usage:   "use public ip instead of private. If vm doesn't have public ip, it will be skipped from the list",
			Value:   false,
		},
	}

	app.RunAndExitOnError()
}

func parseElement(element string) (map[string]string, error) {
	var elementParsed map[string]string
	if err := json.Unmarshal([]byte(element), &elementParsed); err != nil {
		return elementParsed, err
	}

	return elementParsed, nil

}

func promptSelect(menuElements []string, elementKeys string, elementPrintPattern string) (string, error) {
	var selectionSymbol string

	searcher := func(input string, index int) bool {
		element, err := parseElement(menuElements[index])
		if err != nil {
			return false
		}

		name := strings.Replace(strings.ToLower(element["Name"]), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)

		return strings.Contains(name, input)
	}

	promptui.IconInitial = ""

	if runtime.GOOS == "windows" {
		selectionSymbol = "->"
	} else {
		selectionSymbol = "➜"
	}

	var funcMap = promptui.FuncMap
	funcMap["parse"] = parseElement

	templates := promptui.SelectTemplates{
		Label:    "{{ . | cyan }}",
		Active:   fmt.Sprintf("%s %s", selectionSymbol, "{{ (printf \""+elementPrintPattern+"\" "+elementKeys+") | green }}"),
		Inactive: "  {{ (printf \"" + elementPrintPattern + "\" " + elementKeys + ") | white }}",
		FuncMap:  funcMap,
	}

	prompt := promptui.Select{
		Label:        "Select a target (press \"q\" for exit)",
		Items:        menuElements,
		Templates:    &templates,
		Size:         20,
		Searcher:     searcher,
		HideSelected: true,
		Keys: &promptui.SelectKeys{
			Next: promptui.Key{
				Code:    readline.CharNext,
				Display: "↓",
			},
			Prev: promptui.Key{
				Code:    readline.CharPrev,
				Display: "↑",
			},
			PageUp: promptui.Key{
				Code:    readline.CharForward,
				Display: "→",
			},
			PageDown: promptui.Key{
				Code:    readline.CharBackward,
				Display: "←",
			},
			Search: promptui.Key{
				Code:    '/',
				Display: "/",
			},
			Exit: promptui.Key{
				Code:    'q',
				Display: "q",
			},
		},
	}

	_, result, err := prompt.Run()

	if err != nil {
		if err == promptui.ErrInterrupt {
			return result, fmt.Errorf("Interrupted by \"%w\"", err)
		} else if err == promptui.ErrEOF {
			return result, fmt.Errorf("Unexpected end of file: \"%w\"", err)
		} else {
			return result, err
		}
	}

	return result, nil
}

func action(c *cli.Context) error {
	session, err := newSessionFromFile()
	if err != nil {
		return err
	}

	location := c.String("location")
	tags := c.String("tags")
	resourceGroup := c.String("resource-group")
	publicIP := c.Bool("public-ip")

	// Get Azure Resource Group
	if resourceGroup != "" {
		resourceGroupToPassOn = resourceGroup
	} else {
		resourceGroups, err := getResourceGroups(session, location)
		if err != nil {
			return err
		}

		resourceGroupKeys := "(. | parse).Number (. | parse).Name (. | parse).Location"
		resourceGroupPrintPattern := "%v.\t%v (%v)"

		selectedGroup, err := promptSelect(resourceGroups, resourceGroupKeys, resourceGroupPrintPattern)

		if err != nil {
			return err
		}

		parsedSelectedGroup, err := parseElement(selectedGroup)
		if err != nil {
			return err
		}

		resourceGroupToPassOn = parsedSelectedGroup["Name"]
	}

	// Get Azure Virtual Machine
	virtualMachines, err := getVM(session, resourceGroupToPassOn, tags, publicIP)
	if err != nil {
		if err.Error() == "WrongTagDefinition" {
			cli.ShowAppHelp(c)
			return fmt.Errorf("\nIncorrect Usage. Wrong tag definition in flag -t")
		}
		return fmt.Errorf("There was an error listing virtual machines:\n%w", err)
	}

	elementKeys := "(. | parse).Number (. | parse).IP (. | parse).Name"
	elementPrintPattern := "%v. %v\t| %v"

	selectedVirtualMachine, err := promptSelect(virtualMachines, elementKeys, elementPrintPattern)

	if err != nil {
		return err
	}

	parsedSelectedVM, err := parseElement(selectedVirtualMachine)
	if err != nil {
		return err
	}

	ipMatched, err := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+$`, parsedSelectedVM["IP"])
	if err != nil {
		return err
	}

	if !ipMatched {
		return fmt.Errorf("Wrong IP address. Switch to private IP if VM doesn't have public IP")
	}

	sshPath, err := exec.LookPath(sshExecutable)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	cmd := exec.Command(sshPath, parsedSelectedVM["IP"])
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	if err != nil {
		return fmt.Errorf("Command finished with an error, ssh: %w", err)
	}

	return nil
}
