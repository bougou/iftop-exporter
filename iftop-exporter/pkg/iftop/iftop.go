package iftop

import (
	"io"
	"os/exec"
)

type Command struct {
	cmd *exec.Cmd
}

// StdoutPipe returns a pipe that will be connected to the command's
// standard output when the command starts.
func (r Command) StdoutPipe() (io.ReadCloser, error) {
	return r.cmd.StdoutPipe()
}

// StderrPipe returns a pipe that will be connected to the command's
// standard error when the command starts.
func (r Command) StderrPipe() (io.ReadCloser, error) {
	return r.cmd.StderrPipe()
}

// Run start iftop process
func (r Command) Run() error {
	if err := r.cmd.Start(); err != nil {
		return err
	}
	return r.cmd.Wait()
}

// GetCmd return the underlying exec.Cmd.
func (r Command) GetCmd() *exec.Cmd {
	return r.cmd
}

// NewIfTop returns task with described options
func NewIfTop(interfaceName string) *Command {
	binaryPath := "stdbuf"
	arguments := []string{
		"-oL",
		"iftop",
		"-n",
		"-t",
		"-o",
		"10s",
		"-i",
		interfaceName,
	}

	cmd := exec.Command(binaryPath, arguments...)
	return &Command{
		cmd: cmd,
	}
}
