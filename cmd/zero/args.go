package main

import (
	"fmt"
	"strconv"
	"strings"
)

// argSpec says what a command accepts: options without a value, options
// with one, and how many other arguments.
type argSpec struct {
	flags    []string
	values   []string
	min, max int
	names    string
}

var argSpecs = map[string]argSpec{
	"up":         {flags: []string{"--approve", "--vm"}, values: []string{"--max-input"}},
	"attach":     {},
	"code":       {},
	"web":        {},
	"status":     {},
	"stop":       {},
	"logs":       {flags: []string{"-f", "--follow"}},
	"pause":      {},
	"rm":         {flags: []string{"-y"}},
	"ls":         {},
	"switch":     {max: 1, names: "NAME"},
	"completion": {min: 1, max: 1, names: "bash"},
	"install":    {},
	"doctor":     {},
	"version":    {},
	"help":       {},
	"ctx":        {},
	"ctx list":   {},
	"ctx use":    {min: 1, max: 1, names: "NAME"},
	"ctx new":    {min: 1, max: -1, names: "NAME [PATH...]"},
	"ctx add":    {max: -1, names: "[PATH...]"},
	"ctx remove": {max: -1, names: "[PATH...]"},
	"ctx rm":     {flags: []string{"-y"}, min: 1, max: 1, names: "NAME"},
}

// usageError is a mistake in the command line; zero exits with 2.
type usageError string

func (e usageError) Error() string { return string(e) }

// checkArgs refuses what a command does not know, before anything runs.
// The prompt of `run` is free text.
func checkArgs(cmd string, args []string) error {
	if cmd == "run" {
		if len(args) == 0 {
			return usageError("zero run: needs a prompt")
		}
		return nil
	}
	if cmd == "ctx" && len(args) > 0 {
		if args[0] == "--prompt" {
			args = args[1:]
		} else if _, ok := argSpecs["ctx "+args[0]]; ok {
			cmd, args = "ctx "+args[0], args[1:]
		} else {
			return usageError(fmt.Sprintf("zero ctx: unknown subcommand %q (subcommands: use, new, add, remove, rm)", args[0]))
		}
	}
	spec := argSpecs[cmd]
	n := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case contains(spec.flags, a):
		case contains(spec.values, a):
			i++
			if i == len(args) {
				return usageError(fmt.Sprintf("zero %s: %s needs a value", cmd, a))
			}
			if v, err := strconv.Atoi(args[i]); err != nil || v <= 0 {
				return usageError(fmt.Sprintf("zero %s: %s needs a number above 0, got %q", cmd, a, args[i]))
			}
		case strings.HasPrefix(a, "-"):
			return usageError(fmt.Sprintf("zero %s: unknown option %q (options: %s)", cmd, a, spec.options()))
		default:
			n++
		}
	}
	if n < spec.min || spec.max >= 0 && n > spec.max {
		if spec.names == "" {
			return usageError(fmt.Sprintf("zero %s: takes no argument, got %q (options: %s)", cmd, strings.Join(args, " "), spec.options()))
		}
		return usageError(fmt.Sprintf("usage: zero %s %s", cmd, strings.TrimSpace(spec.optionUsage()+" "+spec.names)))
	}
	return nil
}

func (s argSpec) options() string {
	all := append([]string{}, s.flags...)
	for _, v := range s.values {
		all = append(all, v+" N")
	}
	if len(all) == 0 {
		return "none"
	}
	return strings.Join(all, ", ")
}

func (s argSpec) optionUsage() string {
	out := ""
	for _, f := range s.flags {
		out += "[" + f + "] "
	}
	return strings.TrimSpace(out)
}

func contains(list []string, a string) bool {
	for _, x := range list {
		if x == a {
			return true
		}
	}
	return false
}
