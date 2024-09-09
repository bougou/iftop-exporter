package iftop

import (
	"bytes"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/bougou/go-unit"
)

var (
	// 11 10.96.225.7:6801      =>     1.79Kb     9.66Kb     7.63Kb     38.1KB
	flowOutPattern = `(?P<Index>\d+)\s*(?P<Addr>\S+)\s*=>\s*(?P<Last2>\S+)\s*(?P<Last10>\S+)\s*(?P<Last40>\S+)\s*(?P<Cumulative>\S+)`
	flowOutMatcher = regexp.MustCompile(flowOutPattern)

	//    10.0.10.203:34846     <=     1.85Kb      376Kb      327Kb     1.59MB
	flowInPattern = `(?P<Addr>\S+)\s*<=\s*(?P<Last2>\S+)\s*(?P<Last10>\S+)\s*(?P<Last40>\S+)\s*(?P<Cumulative>\S+)`
	flowInMatcher = regexp.MustCompile(flowInPattern)
)

func (task *Task) processStderrLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if strings.HasPrefix(line, `#`) || strings.HasPrefix(line, `-`) || strings.HasPrefix(line, `=`) {
		return
	}

	if strings.HasPrefix(line, "interface:") {
		interfaceName, _ := strings.CutPrefix(line, "interface:")
		task.state.Interface = strings.TrimSpace(interfaceName)
		return
	}

	if strings.HasPrefix(line, "IP address is:") {
		ipv4, _ := strings.CutPrefix(line, "IP address is:")
		task.state.IP = strings.TrimSpace(ipv4)
		return
	}

	if strings.HasPrefix(line, "IPv6 address is:") {
		ipv6, _ := strings.CutPrefix(line, "IPv6 address is:")
		task.state.IPv6 = strings.TrimSpace(ipv6)
		return
	}

	if strings.HasPrefix(line, "MAC address is:") {
		mac, _ := strings.CutPrefix(line, "MAC address is:")
		task.state.MAC = strings.TrimSpace(mac)
		return
	}
}

func (task *Task) processStdoutLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if strings.HasPrefix(line, `#`) || strings.HasPrefix(line, `-`) || strings.HasPrefix(line, `=`) {
		return
	}

	if strings.Contains(line, "=>") {
		m, matched := GetNamedCapturingGroupMap(flowOutMatcher, line)
		if !matched {
			return
		}

		index, err := strconv.Atoi(m["Index"])
		if err != nil {
			return
		}

		if index == 1 {
			// init new flowStats
			task.processingFlowStats = &FlowStats{
				Flows: make([]*Flow, 0),
			}
		}

		task.processingIndex = index
		task.processingOutFlow = &Flow{
			Index:           index,
			Src:             m["Addr"],
			Direction:       FlowDirectionOut,
			Type:            FlowTypePublic,
			Last2RateBits:   parseValueToBits(m["Last2"]),
			Last10RateBits:  parseValueToBits(m["Last10"]),
			Last40RateBits:  parseValueToBits(m["Last40"]),
			CumulativeBytes: parseValueToBits(m["Cumulative"]) / 8,
		}

		return
	}

	if strings.Contains(line, "<=") {
		if task.processingIndex == 0 {
			// no toFlow saved
			return
		}

		m, matched := GetNamedCapturingGroupMap(flowInMatcher, line)
		if !matched {
			return
		}

		outFlow := task.processingOutFlow
		if outFlow == nil {
			return
		}
		outFlow.Dst = m["Addr"]

		inFlow := &Flow{
			Index:           outFlow.Index,
			Src:             outFlow.Src,
			Dst:             m["Addr"],
			Direction:       FlowDirectionIn,
			Type:            FlowTypePublic,
			Last2RateBits:   parseValueToBits(m["Last2"]),
			Last10RateBits:  parseValueToBits(m["Last10"]),
			Last40RateBits:  parseValueToBits(m["Last40"]),
			CumulativeBytes: parseValueToBits(m["Cumulative"]) / 8,
		}

		srcIP := extractIP(outFlow.Src)
		dstIP := extractIP(outFlow.Dst)
		if net.ParseIP(srcIP).IsPrivate() && net.ParseIP(dstIP).IsPrivate() {
			outFlow.Type = FlowTypePrivate
			inFlow.Type = FlowTypePrivate
		}

		if task.processingFlowStats != nil {
			task.processingFlowStats.Flows = append(task.processingFlowStats.Flows, outFlow, inFlow)
		}
		return
	}

	if strings.HasPrefix(line, "Total send rate:") {
		line, _ = strings.CutPrefix(line, "Total send rate:")
		line = strings.TrimSpace(line)
		words := strings.Fields(strings.TrimSpace(line))

		if len(words) != 3 {
			return
		}
		if task.processingFlowStats != nil {
			task.processingFlowStats.TotalSentLast2RateBits = parseValueToBits(words[0])
			task.processingFlowStats.TotalSentLast10RateBits = parseValueToBits(words[1])
			task.processingFlowStats.TotalSentLast40RateBits = parseValueToBits(words[2])
		}
		return
	}

	if strings.HasPrefix(line, "Total receive rate:") {
		line, _ = strings.CutPrefix(line, "Total receive rate:")
		line = strings.TrimSpace(line)
		words := strings.Fields(strings.TrimSpace(line))

		if len(words) != 3 {
			return
		}
		if task.processingFlowStats != nil {
			task.processingFlowStats.TotalRecvLast2RateBits = parseValueToBits(words[0])
			task.processingFlowStats.TotalRecvLast10RateBits = parseValueToBits(words[1])
			task.processingFlowStats.TotalRecvLast40RateBits = parseValueToBits(words[2])
		}

		return
	}

	if strings.HasPrefix(line, "Total send and receive rate:") {
		line, _ = strings.CutPrefix(line, "Total send and receive rate:")
		line = strings.TrimSpace(line)
		words := strings.Fields(strings.TrimSpace(line))
		if len(words) != 3 {
			return
		}
		if task.processingFlowStats != nil {
			task.processingFlowStats.TotalSentAndRecvLast2RateBits = parseValueToBits(words[0])
			task.processingFlowStats.TotalSentAndRecvLast10RateBits = parseValueToBits(words[1])
			task.processingFlowStats.TotalSentAndRecvLast40RateBits = parseValueToBits(words[2])
		}
		return
	}

	if strings.HasPrefix(line, "Peak rate (sent/received/total):") {
		line, _ = strings.CutPrefix(line, "Peak rate (sent/received/total):")
		line = strings.TrimSpace(line)
		words := strings.Fields(strings.TrimSpace(line))

		if len(words) != 3 {
			return
		}
		if task.processingFlowStats != nil {
			task.processingFlowStats.PeakSentRateBits = parseValueToBits(words[0])
			task.processingFlowStats.PeakRecvRateBits = parseValueToBits(words[1])
			task.processingFlowStats.PeakSentAndRecvRateBits = parseValueToBits(words[2])
		}
		return
	}

	if strings.HasPrefix(line, "Cumulative (sent/received/total):") {
		line, _ = strings.CutPrefix(line, "Cumulative (sent/received/total):")
		line = strings.TrimSpace(line)
		words := strings.Fields(strings.TrimSpace(line))

		if len(words) != 3 {
			return
		}
		if task.processingFlowStats != nil {
			task.processingFlowStats.CumulativeSentBytes = parseValueToBits(words[0]) / 8
			task.processingFlowStats.CumulativeRecvBytes = parseValueToBits(words[1]) / 8
			task.processingFlowStats.CumulativeSentAndRecvBytes = parseValueToBits(words[2]) / 8
		}

		// Now, the process for this round finished, saving the flowStats.
		task.state.FlowStats = task.processingFlowStats
		return
	}

}

func parseValueToBits(value string) (bits float64) {
	bitOrByte := "bit"
	if strings.HasSuffix(value, `B`) {
		bitOrByte = "byte"
	}
	value, _ = strings.CutSuffix(value, `b`)
	value, _ = strings.CutSuffix(value, `B`)

	v, err := unit.PrefixParse(value, unit.SI1024)
	if err != nil {
		return 0
	}

	if bitOrByte == "byte" {
		return v * 8
	}

	return v
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
