package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zach/huddle/internal/config"
	"github.com/zach/huddle/internal/engine"
	"github.com/zach/huddle/internal/model"
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return errors.New("expected plan, show, or apply")
	}
	switch args[0] {
	case "plan":
		return runPlan(args[1:], stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "apply":
		return runApply(args[1:], stdin, stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runPlan(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "configuration YAML file")
	fs.StringVar(file, "f", "", "configuration YAML file")
	out := fs.String("out", "", "saved plan path")
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return errors.New("plan requires --file")
	}
	cfg, raw, err := config.Load(*file)
	if err != nil {
		return err
	}
	plan, err := engine.BuildPlan(*file, cfg, raw)
	if err != nil {
		return err
	}
	if *out != "" {
		if err := writePlan(*out, plan); err != nil {
			return err
		}
	}
	if *jsonOutput {
		return json.NewEncoder(stdout).Encode(plan)
	}
	printPlan(stdout, plan)
	if *out != "" {
		fmt.Fprintf(stdout, "\nPlan saved to %s\n", *out)
	}
	return nil
}

func runShow(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("show requires a plan file")
	}
	plan, err := readPlan(fs.Arg(0))
	if err != nil {
		return err
	}
	if *jsonOutput {
		return json.NewEncoder(stdout).Encode(plan)
	}
	printPlan(stdout, plan)
	return nil
}

func runApply(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(stderr)
	yes := fs.Bool("yes", false, "apply without prompting")
	fs.BoolVar(yes, "y", false, "apply without prompting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("apply requires a plan file")
	}
	plan, err := readPlan(fs.Arg(0))
	if err != nil {
		return err
	}
	printPlan(stdout, plan)
	if !*yes {
		fmt.Fprint(stdout, "\nApply this plan? [y/N] ")
		var answer string
		fmt.Fscanln(stdin, &answer)
		if strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
			fmt.Fprintln(stdout, "Apply cancelled.")
			return nil
		}
	}
	if err := engine.Apply(plan); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Apply complete.")
	return nil
}

func writePlan(path string, plan model.Plan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}
func readPlan(path string) (model.Plan, error) {
	var p model.Plan
	data, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	if p.Version != 1 {
		return p, fmt.Errorf("unsupported plan version %d", p.Version)
	}
	return p, nil
}

func printPlan(w io.Writer, plan model.Plan) {
	if len(plan.Changes) == 0 {
		fmt.Fprintln(w, "No changes.")
		return
	}
	for _, c := range plan.Changes {
		fmt.Fprintf(w, "%s %s %s\n", strings.ToUpper(strings.Join(c.Actions, ",")), c.Type, c.Target)
		if c.Reason != "" {
			fmt.Fprintf(w, "  reason: %s\n", c.Reason)
		}
		if c.Diff != "" {
			for _, line := range strings.Split(c.Diff, "\n") {
				fmt.Fprintf(w, "  %s\n", line)
			}
		}
	}
	fmt.Fprintf(w, "\nPlan: %d resources to change.\n", len(plan.Changes))
}

func usage(w io.Writer) { fmt.Fprintln(w, "usage: huddle <plan|show|apply> [options]") }
