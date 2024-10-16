package iftop

import (
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

		// invalid, the index of iftop flow starts from 1
		if index == 0 {
			return
		}

		if index == 1 {
			// set flag to indicate that index-1 flow is found
			// and ensure the following fields are initialized
			task.flowIndex1Found = true

			// init new flowStats
			task.processingFlowStats = &FlowStats{
				Flows: make([]*Flow, 0),
			}

			// init sum flows
			task.sumPrivateInFlow = &Flow{
				Src:             "all",
				Dst:             "all",
				Direction:       FlowDirectionIn,
				Type:            FlowTypePrivate,
				Last2RateBits:   0,
				Last10RateBits:  0,
				Last40RateBits:  0,
				CumulativeBytes: 0,
			}
			task.sumPrivateOutFlow = &Flow{
				Src:             "all",
				Dst:             "all",
				Direction:       FlowDirectionOut,
				Type:            FlowTypePrivate,
				Last2RateBits:   0,
				Last10RateBits:  0,
				Last40RateBits:  0,
				CumulativeBytes: 0,
			}
			task.sumPublicInFlow = &Flow{
				Src:             "all",
				Dst:             "all",
				Direction:       FlowDirectionIn,
				Type:            FlowTypePublic,
				Last2RateBits:   0,
				Last10RateBits:  0,
				Last40RateBits:  0,
				CumulativeBytes: 0,
			}
			task.sumPublicOutFlow = &Flow{
				Src:             "all",
				Dst:             "all",
				Direction:       FlowDirectionOut,
				Type:            FlowTypePublic,
				Last2RateBits:   0,
				Last10RateBits:  0,
				Last40RateBits:  0,
				CumulativeBytes: 0,
			}
		}

		if !task.flowIndex1Found {
			return
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
		// this is the `in` flow

		if task.processingIndex == 0 {
			// no outFlow saved, ignore inFlow
			return
		}

		if !task.flowIndex1Found {
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
			task.sumPrivateOutFlow.Last2RateBits += outFlow.Last2RateBits
			task.sumPrivateOutFlow.Last10RateBits += outFlow.Last10RateBits
			task.sumPrivateOutFlow.Last40RateBits += outFlow.Last40RateBits
			task.sumPrivateOutFlow.CumulativeBytes += outFlow.CumulativeBytes

			inFlow.Type = FlowTypePrivate
			task.sumPrivateInFlow.Last2RateBits += inFlow.Last2RateBits
			task.sumPrivateInFlow.Last10RateBits += inFlow.Last10RateBits
			task.sumPrivateInFlow.Last40RateBits += inFlow.Last40RateBits
			task.sumPrivateInFlow.CumulativeBytes += inFlow.CumulativeBytes
		} else {
			outFlow.Type = FlowTypePublic
			task.sumPublicOutFlow.Last2RateBits += outFlow.Last2RateBits
			task.sumPublicOutFlow.Last10RateBits += outFlow.Last10RateBits
			task.sumPublicOutFlow.Last40RateBits += outFlow.Last40RateBits
			task.sumPublicOutFlow.CumulativeBytes += outFlow.CumulativeBytes

			inFlow.Type = FlowTypePublic
			task.sumPublicInFlow.Last2RateBits += inFlow.Last2RateBits
			task.sumPublicInFlow.Last10RateBits += inFlow.Last10RateBits
			task.sumPublicInFlow.Last40RateBits += inFlow.Last40RateBits
			task.sumPublicInFlow.CumulativeBytes += inFlow.CumulativeBytes
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

			if len(task.processingFlowStats.Flows) > 0 {
				task.processingFlowStats.Flows = append(task.processingFlowStats.Flows,
					task.sumPrivateInFlow,
					task.sumPrivateOutFlow,
					task.sumPublicInFlow,
					task.sumPublicOutFlow)
			}
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
