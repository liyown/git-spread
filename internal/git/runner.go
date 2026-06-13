package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Runner struct {
	Dir string
}

func NewRunner(dir string) Runner {
	return Runner{Dir: dir}
}

func (r Runner) Run(args ...string) error {
	_, err := r.run(args...)
	return err
}

func (r Runner) Output(args ...string) (string, error) {
	return r.run(args...)
}

func (r Runner) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
