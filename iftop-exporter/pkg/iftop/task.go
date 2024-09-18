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
	processingIndex     int
	processingOutFlow   *Flow
	processingFlowStats *FlowStats
}

// Log contains raw stderr and stdout outputs
type Log struct {
	Stderr string `json:"stderr"`
	Stdout string `json:"stdout"`
}

func NewTask(options Options) *Task {
	options.useTextMode = true

	return &Task{
		iftop: NewIftop(options),
		state: &State{},
		log:   &Log{},
	}
}

// State returns information about progress task
func (t Task) State() State {
	return *t.state
}

// Log return structure which contains raw stderr and stdout outputs
func (t Task) Log() Log {
	return Log{
		Stderr: t.log.Stderr,
		Stdout: t.log.Stdout,
	}
}

func (t Task) ID() string {
	return t.iftop.options.InterfaceName
}

// String return the actual exec cmd string of the task
func (t Task) String() string {
	return t.iftop.cmd.String()
}

// Run starts and waits the program until exit, and also process stdout/stderr in other go-routines.
func (t *Task) Run() error {
	var err error

	// The pipe would be auto closed by `Wait`, so the caller that uses the pipe does not need to close it.
	stderr, err := t.iftop.StderrPipe()
	if err != nil {
		return fmt.Errorf("get StderrPipe failed, err: %s", err)
	}
	defer stderr.Close()

	stdout, err := t.iftop.StdoutPipe()
	if err != nil {
		return fmt.Errorf("get StdoutPipe failed, err: %s", err)
	}
	defer stdout.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go processStdout(&wg, t, stdout)
	go processStderr(&wg, t, stderr)

	err = t.iftop.Run()
	wg.Wait()

	return err
}

// GetCmd return the underlying exec.Cmd.
func (r Task) GetCmd() *exec.Cmd {
	return r.iftop.cmd
}

func processStdout(wg *sync.WaitGroup, task *Task, stdout io.Reader) {
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

func processStderr(wg *sync.WaitGroup, task *Task, stderr io.Reader) {
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
