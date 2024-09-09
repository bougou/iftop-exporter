package iftop

import (
	"bufio"
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

type State struct {
	Interface string     `json:"interface"`
	IP        string     `json:"ip"`
	IPv6      string     `json:"ipv6"`
	MAC       string     `json:"mac"`
	FlowStats *FlowStats `json:"flow_stats"`
}

type FlowStats struct {
	Flows []*Flow `json:"flows"`

	TotalSentLast2RateBits  float64 // unit: bits per second
	TotalSentLast10RateBits float64 // unit: bits per second
	TotalSentLast40RateBits float64 // unit: bits per second

	TotalRecvLast2RateBits  float64 // unit: bits per second
	TotalRecvLast10RateBits float64 // unit: bits per second
	TotalRecvLast40RateBits float64 // unit: bits per second

	TotalSentAndRecvLast2RateBits  float64 // unit: bits per second
	TotalSentAndRecvLast10RateBits float64 // unit: bits per second
	TotalSentAndRecvLast40RateBits float64 // unit: bits per second

	PeakSentRateBits        float64 // unit: bits per second
	PeakRecvRateBits        float64 // unit: bits per second
	PeakSentAndRecvRateBits float64 // unit: bits per second

	CumulativeSentBytes        float64 // unit: Bytes
	CumulativeRecvBytes        float64 // unit: Bytes
	CumulativeSentAndRecvBytes float64 // unit: Bytes
}

type FlowDirection string

const (
	FlowDirectionOut FlowDirection = "out" // src => dst
	FlowDirectionIn  FlowDirection = "in"  // src <= dst
	FlowDirectionX   FlowDirection = "x"   // src <=> dst (in and out)
)

type FlowType string

const (
	FlowTypePublic  FlowType = "public"
	FlowTypePrivate FlowType = "private"
)

type Flow struct {
	Index     int
	Src       string
	Dst       string
	Direction FlowDirection
	Type      FlowType

	Last2RateBits   float64 // unit: bits per second
	Last10RateBits  float64 // unit: bits per second
	Last40RateBits  float64 // unit: bits per second
	CumulativeBytes float64 // unit: Bytes
}

func NewTask(interfaceName string) *Task {
	return &Task{
		iftop: NewIfTop(interfaceName),
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

// String return the actual exec cmd string of the task
func (t Task) String() string {
	return t.iftop.cmd.String()
}

// Run starts and waits the program until exi, and also process stdout/stderr in other go-routines.
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

func removeAllEscape(s string) string {
	// ANSI 转义序列的正则表达式模式
	pattern := `\x1b\[[0-9;]*[a-zA-Z]`
	reg := regexp.MustCompile(pattern)
	return reg.ReplaceAllString(s, "")
}
