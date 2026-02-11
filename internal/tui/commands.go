package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

func runCommandSeries(writer io.Writer, dir string, cmds ...*exec.Cmd) error {
	for _, cmd := range cmds {
		cmd.Dir = dir
		if err := runCommandStreaming(writer, cmd); err != nil {
			return err
		}
	}
	return nil
}

func runCommandStreaming(writer io.Writer, cmd *exec.Cmd) error {
	_, _ = fmt.Fprintf(writer, "$ %s %s\n", cmd.Path, strings.Join(cmd.Args[1:], " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamReader(writer, stdout, &wg)
	go streamReader(writer, stderr, &wg)
	wg.Wait()

	return cmd.Wait()
}

func streamReader(writer io.Writer, reader io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		_, _ = fmt.Fprintln(writer, scanner.Text())
	}
}

func buildCommands(ipsw string) []*exec.Cmd {
	return []*exec.Cmd{
		exec.Command("packer", "init", "runner.pkr.hcl"),
		exec.Command("packer", "build", "runner.pkr.hcl"),
	}
}

func listTartVMPaths() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	glob := filepath.Join(homeDir, ".tart", "vms", "*")
	paths, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	return paths, nil
}
