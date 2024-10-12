package manager

import (
	"github.com/bougou/iftop-exporter/iftop-exporter/pkg/iftop"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	flowLast2 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_flow_last2_speed_bps",
		Help: "data transfer rate (bits per second) of the flow over the preceding 2 seconds",
	}, []string{"interface", "src", "dst", "direction", "type", "owner"})

	flowLast10 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_flow_last10_speed_bps",
		Help: "data transfer rate (bits per second) of the flow over the preceding 10 seconds",
	}, []string{"interface", "src", "dst", "direction", "type", "owner"})

	flowLast40 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_flow_last40_speed_bps",
		Help: "data transfer rate (bits per second) of the flow over the preceding 40 seconds",
	}, []string{"interface", "src", "dst", "direction", "type", "owner"})

	flowCumulative = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_flow_cumulative_bytes",
		Help: "cumulative bytes of the flow",
	}, []string{"interface", "src", "dst", "direction", "type", "owner"})

	totalLast2 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_total_last2_speed_bps",
		Help: "data transfer rate (bits per second) of all flows over the preceding 2 seconds",
	}, []string{"interface", "direction", "owner"})

	totalLast10 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_total_last10_speed_bps",
		Help: "data transfer rate (bits per second) of all flows over the preceding 10 seconds",
	}, []string{"interface", "direction", "owner"})

	totalLast40 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_total_last40_speed_bps",
		Help: "data transfer rate (bits per second) of all flows over the preceding 40 seconds",
	}, []string{"interface", "direction", "owner"})

	peak = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_peak_speed_bps",
		Help: "the peak data transfer rate (bits per second) of all flows",
	}, []string{"interface", "direction", "owner"})

	cumulative = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "iftop_cumulative_bytes",
		Help: "the cumulative bytes of all flows",
	}, []string{"interface", "direction", "owner"})
)

// updateMetrics update metrics by reading the value from state
func (mgr *Manager) updateMetrics(states []iftop.State) {
	if len(states) == 0 {
		return
	}

	flowLast2.Reset()
	flowLast10.Reset()
	flowLast40.Reset()
	flowCumulative.Reset()
	totalLast2.Reset()
	totalLast10.Reset()
	totalLast40.Reset()
	peak.Reset()
	cumulative.Reset()

	for _, state := range states {
		if state.FlowStats == nil {
			continue
		}

		interfaceName := state.Interface
		out := string(iftop.FlowDirectionOut)
		in := string(iftop.FlowDirectionIn)
		x := string(iftop.FlowDirectionX)
		interfaceInfo := mgr.dynamicInterfaceInfo[interfaceName]
		owner := interfaceInfo["owner"]

		mgr.Debugf("update metrics: (%d) flows for interface (%s, %s)", len(state.FlowStats.Flows), interfaceName, owner)

		for _, flow := range state.FlowStats.Flows {
			if flow == nil {
				continue
			}

			src := flow.Src
			dst := flow.Dst
			direction := string(flow.Direction)
			flowType := string(flow.Type)
			if src == "" || dst == "" {
				continue
			}

			flowLast2.WithLabelValues(interfaceName, src, dst, direction, flowType, owner).Set(flow.Last2RateBits)
			flowLast10.WithLabelValues(interfaceName, src, dst, direction, flowType, owner).Set(flow.Last10RateBits)
			flowLast40.WithLabelValues(interfaceName, src, dst, direction, flowType, owner).Set(flow.Last40RateBits)
			flowCumulative.WithLabelValues(interfaceName, src, dst, direction, flowType, owner).Set(flow.CumulativeBytes)
		}

		totalLast2.WithLabelValues(interfaceName, out, owner).Set(state.FlowStats.TotalSentLast2RateBits)
		totalLast2.WithLabelValues(interfaceName, in, owner).Set(state.FlowStats.TotalRecvLast2RateBits)
		totalLast2.WithLabelValues(interfaceName, x, owner).Set(state.FlowStats.TotalSentAndRecvLast2RateBits)

		totalLast10.WithLabelValues(interfaceName, out, owner).Set(state.FlowStats.TotalSentLast10RateBits)
		totalLast10.WithLabelValues(interfaceName, in, owner).Set(state.FlowStats.TotalRecvLast10RateBits)
		totalLast10.WithLabelValues(interfaceName, x, owner).Set(state.FlowStats.TotalSentAndRecvLast10RateBits)

		totalLast40.WithLabelValues(interfaceName, out, owner).Set(state.FlowStats.TotalSentLast40RateBits)
		totalLast40.WithLabelValues(interfaceName, in, owner).Set(state.FlowStats.TotalRecvLast40RateBits)
		totalLast40.WithLabelValues(interfaceName, x, owner).Set(state.FlowStats.TotalSentAndRecvLast40RateBits)

		peak.WithLabelValues(interfaceName, out, owner).Set(state.FlowStats.PeakSentRateBits)
		peak.WithLabelValues(interfaceName, in, owner).Set(state.FlowStats.PeakRecvRateBits)
		peak.WithLabelValues(interfaceName, x, owner).Set(state.FlowStats.PeakSentAndRecvRateBits)

		cumulative.WithLabelValues(interfaceName, out, owner).Set(state.FlowStats.CumulativeSentBytes)
		cumulative.WithLabelValues(interfaceName, in, owner).Set(state.FlowStats.CumulativeRecvBytes)
		cumulative.WithLabelValues(interfaceName, x, owner).Set(state.FlowStats.CumulativeSentAndRecvBytes)
	}
}
