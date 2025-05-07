package iftop

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

type Task struct {
	iftop               *Command
	state               *State
	log                 *Log
	flowIndex1Found     bool
	processingIndex     int
	processingOutFlow   *Flow
	processingFlowStats *FlowStats
	sumPrivateInFlow    *Flow
	sumPrivateOutFlow   *Flow
	sumPublicInFlow     *Flow
	sumPublicOutFlow    *Flow
}

// Log contains raw stderr and stdout outputs
type Log struct {
	Stderr string `json:"stderr"`
	Stdout string `json:"stdout"`
}

func NewTask(options Options) *Task {
	// useTextMode should always be true
	options.useTextMode = true

	return &Task{
		iftop: NewIftop(options),
		state: &State{},
		log:   &Log{},
	}
}

// State returns information about progress task
func (task Task) State() State {
	return *task.state
}

// Log return structure which contains raw stderr and stdout outputs
func (task Task) Log() Log {
	return Log{
		Stderr: task.log.Stderr,
		Stdout: task.log.Stdout,
	}
}

func (task Task) ID() string {
	return task.iftop.options.InterfaceName
}

// String return the actual exec cmd string of the task
func (task Task) String() string {
	return task.iftop.cmd.String()
}

// Run starts and waits the program until exit, and also process stdout/stderr in other go-routines.
func (task *Task) Run() error {
	var err error

	// The pipe would be auto closed by `Wait`, so the caller that uses the pipe does not need to close it.
	stderr, err := task.iftop.StderrPipe()
	if err != nil {
		return fmt.Errorf("get StderrPipe failed, err: %s", err)
	}
	defer stderr.Close()

	stdout, err := task.iftop.StdoutPipe()
	if err != nil {
		return fmt.Errorf("get StdoutPipe failed, err: %s", err)
	}
	defer stdout.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go task.processStdout(&wg, stdout)
	go task.processStderr(&wg, stderr)

	err = task.iftop.Run()
	wg.Wait()

	return err
}

// GetCmd return the underlying exec.Cmd.
func (task Task) GetCmd() *exec.Cmd {
	return task.iftop.cmd
}

func (task *Task) processStdout(wg *sync.WaitGroup, stdout io.Reader) {
	defer wg.Done()

	scanner := bufio.NewScanner(stdout)
	scanner.Split(scanProgressLines)
	for scanner.Scan() {
		raw := scanner.Text()
		// task.log.Stdout += raw + "\n"

		// the progress output contains escape characters
		line := removeAllEscape(strings.TrimSpace(raw))
		task.processStdoutLine(line)
	}

}

func (task *Task) processStderr(wg *sync.WaitGroup, stderr io.Reader) {
	defer wg.Done()

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		raw := scanner.Text()
		// task.log.Stderr += raw + "\n"
		// the progress output contains escape characters
		line := removeAllEscape(strings.TrimSpace(raw))
		task.processStderrLine(line)
	}
}

func scanProgressLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\n' {
			// We have a line terminated by single newline.
			return i + 1, data[0:i], nil
		}
		advance = i + 1
		if len(data) > i+1 && data[i+1] == '\n' {
			advance += 1
		}
		return advance, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func removeAllEscape(s string) string {
	// ANSI 转义序列的正则表达式模式
	pattern := `\x1b\[[0-9;]*[a-zA-Z]`
	reg := regexp.MustCompile(pattern)
	return reg.ReplaceAllString(s, "")
}
