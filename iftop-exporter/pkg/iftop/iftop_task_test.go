package iftop

import (
	"bytes"
	"sync"
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

func Test_matchProcessInfo(t *testing.T) {

	tests := []struct {
		input    string
		expected State
	}{

		{
			input: `
interface: eno2
IP address is: 10.0.10.201
IPv6 address is: ::
MAC address is: d4:5d:64:bc:bd:4c
Listening on eno2
   # Host name (port/service if enabled)            last 2s   last 10s   last 40s cumulative
--------------------------------------------------------------------------------------------
   1 10.0.10.201:36674                        =>     7.52Kb     7.52Kb     7.52Kb     1.88KB
     10.0.10.204:http                         <=     7.19Mb     7.19Mb     7.19Mb     1.80MB
   2 10.96.225.10:6801                        =>     33.8Kb     33.8Kb     33.8Kb     8.46KB
     10.0.10.205:37870                        <=     6.12Mb     6.12Mb     6.12Mb     1.53MB
   3 10.96.225.10:6802                        =>     5.91Mb     5.91Mb     5.91Mb     1.48MB
     10.96.189.17:55440                       <=     22.8Kb     22.8Kb     22.8Kb     5.69KB
   4 10.96.225.7:6802                         =>     15.3Kb     15.3Kb     15.3Kb     3.82KB
     10.96.189.16:50976                       <=     1.37Mb     1.37Mb     1.37Mb      351KB
   5 10.96.225.12:6802                        =>     10.9Kb     10.9Kb     10.9Kb     2.72KB
     10.96.189.16:34644                       <=      369Kb      369Kb      369Kb     92.2KB
   6 10.96.225.10:46770                       =>      280Kb      280Kb      280Kb     69.9KB
     10.96.189.14:6802                        <=     19.5Kb     19.5Kb     19.5Kb     4.87KB
   7 10.96.225.11:6802                        =>     8.98Kb     8.98Kb     8.98Kb     2.24KB
     10.96.189.13:54496                       <=      215Kb      215Kb      215Kb     53.8KB
   8 10.96.225.11:6802                        =>     10.3Kb     10.3Kb     10.3Kb     2.58KB
     10.96.189.16:51936                       <=      175Kb      175Kb      175Kb     43.6KB
   9 10.96.225.10:51962                       =>     9.73Kb     9.73Kb     9.73Kb     2.43KB
     10.96.189.16:6802                        <=      174Kb      174Kb      174Kb     43.5KB
  10 10.96.225.7:6802                         =>      133Kb      133Kb      133Kb     33.2KB
     10.96.189.14:55346                       <=     5.51Kb     5.51Kb     5.51Kb     1.38KB
--------------------------------------------------------------------------------------------
Total send rate:                                     7.10Mb     7.10Mb     7.10Mb
Total receive rate:                                  16.2Mb     16.2Mb     16.2Mb
Total send and receive rate:                         23.3Mb     23.3Mb     23.3Mb
--------------------------------------------------------------------------------------------
Peak rate (sent/received/total):                     7.10Mb     16.2Mb     23.3Mb
Cumulative (sent/received/total):                    1.78MB     4.04MB     5.82MB
============================================================================================

   # Host name (port/service if enabled)            last 2s   last 10s   last 40s cumulative
--------------------------------------------------------------------------------------------
   1 10.0.10.201:36676                        =>     4.27Kb     4.88Kb     4.88Kb     4.88KB
     10.0.10.204:http                         <=     5.70Mb     6.55Mb     6.55Mb     6.55MB
   2 10.0.10.201:6443                         =>     1.82Mb     1.28Mb     1.28Mb     1.28MB
     10.0.10.203:22461                        <=     2.17Mb     1.56Mb     1.56Mb     1.56MB
   3 10.96.225.12:6801                        =>     27.2Kb     19.4Kb     19.4Kb     19.4KB
     10.0.10.205:40470                        <=     5.11Mb     1.47Mb     1.47Mb     1.47MB
   4 10.96.225.12:6802                        =>     4.89Mb     1.26Mb     1.26Mb     1.26MB
     10.96.189.16:34644                       <=     44.7Kb     37.8Kb     37.8Kb     37.8KB
   5 10.96.225.10:51962                       =>      553Kb      785Kb      785Kb      785KB
     10.96.189.16:6802                        <=     12.9Kb     37.6Kb     37.6Kb     37.6KB
   6 10.96.225.10:6801                        =>     9.55Kb     23.2Kb     23.2Kb     23.2KB
     10.0.10.205:37870                        <=      523Kb      723Kb      723Kb      723KB
   7 10.96.225.12:6802                        =>      280Kb      282Kb      282Kb      282KB
     10.96.189.17:57956                       <=     12.4Kb      341Kb      341Kb      341KB
   8 10.96.225.11:6802                        =>     3.84Kb     5.70Kb     5.70Kb     5.70KB
     10.96.189.16:51936                       <=     3.48Kb      485Kb      485Kb      485KB
   9 10.96.225.7:6802                         =>     1.50Kb     32.5Kb     32.5Kb     32.5KB
     10.96.189.14:55346                       <=     1.36Kb      423Kb      423Kb      423KB
  10 10.96.225.11:6802                        =>     2.48Kb     4.40Kb     4.40Kb     4.40KB
     10.96.189.13:54496                       <=     2.25Kb      355Kb      355Kb      355KB
--------------------------------------------------------------------------------------------
Total send rate:                                     7.85Mb     5.32Mb     5.32Mb
Total receive rate:                                  13.9Mb     13.2Mb     13.2Mb
Total send and receive rate:                         21.8Mb     18.5Mb     18.5Mb
--------------------------------------------------------------------------------------------
Peak rate (sent/received/total):                     8.24Mb     23.6Mb     31.9Mb
Cumulative (sent/received/total):                    5.32MB     13.2MB     18.5MB
============================================================================================
			`,
			expected: State{
				Interface: "eno2",
				FlowStats: &FlowStats{
					Flows: []*Flow{
						{
							Index:          1,
							Src:            "10.0.10.201:36676",
							Dst:            "10.0.10.204:http",
							Direction:      FlowDirectionOut,
							Last2RateBits:  4.27 * 1024,
							Last10RateBits: 4.88 * 1024,
							Last40RateBits: 4.88 * 1024,
						},
						{
							Index:          1,
							Src:            "10.0.10.201:36676",
							Dst:            "10.0.10.204:http",
							Direction:      FlowDirectionIn,
							Last2RateBits:  5.70 * 1024 * 1024,
							Last10RateBits: 6.55 * 1024 * 1024,
							Last40RateBits: 6.55 * 1024 * 1024,
						},
						{Index: 2},
						{Index: 2},
						{Index: 3},
						{Index: 3},
						{Index: 4},
						{Index: 4},
						{Index: 5},
						{Index: 5},
						{Index: 6},
						{Index: 6},
						{Index: 7},
						{Index: 7},
						{Index: 8},
						{Index: 8},
						{Index: 9},
						{Index: 9},
						{
							Index:          10,
							Src:            "10.96.225.11:6802",
							Dst:            "10.96.189.13:54496",
							Direction:      FlowDirectionOut,
							Last2RateBits:  2.48 * 1024,
							Last10RateBits: 4.40 * 1024,
							Last40RateBits: 4.40 * 1024,
						},
						{
							Index:          10,
							Src:            "10.96.225.11:6802",
							Dst:            "10.96.189.13:54496",
							Direction:      FlowDirectionIn,
							Last2RateBits:  2.25 * 1024,
							Last10RateBits: 355 * 1024,
							Last40RateBits: 355 * 1024,
						},
					},
				},
			},
		},
	}

	task := NewTask("eno2")

	var wg sync.WaitGroup

	for _, tt := range tests {
		reader := bytes.NewReader([]byte(tt.input))
		wg.Add(1)
		processStdout(&wg, task, reader)
		pretty.Println(task.state)

		assert.Equal(t, "eno2", task.state.Interface)

		flow1 := task.state.FlowStats.Flows[0]
		flow1Expected := tt.expected.FlowStats.Flows[0]

		assert.Equal(t, flow1Expected.Direction, flow1.Direction)
		assert.Equal(t, flow1Expected.Last2RateBits, flow1.Last2RateBits)
		assert.Equal(t, flow1Expected.Last10RateBits, flow1.Last10RateBits)
		assert.Equal(t, flow1Expected.Last40RateBits, flow1.Last40RateBits)

		flow19 := task.state.FlowStats.Flows[18]
		flow19Expected := tt.expected.FlowStats.Flows[18]
		assert.Equal(t, flow19Expected.Direction, flow19.Direction)
		assert.Equal(t, flow19Expected.Last2RateBits, flow19.Last2RateBits)
		assert.Equal(t, flow19Expected.Last10RateBits, flow19.Last10RateBits)
		assert.Equal(t, flow19Expected.Last40RateBits, flow19.Last40RateBits)

		flow20 := task.state.FlowStats.Flows[19]
		flow20Expected := tt.expected.FlowStats.Flows[19]
		assert.Equal(t, flow20Expected.Direction, flow20.Direction)
		assert.Equal(t, flow20Expected.Last2RateBits, flow20.Last2RateBits)
		assert.Equal(t, flow20Expected.Last10RateBits, flow20.Last10RateBits)
		assert.Equal(t, flow20Expected.Last40RateBits, flow20.Last40RateBits)
	}

}

func Test_removeAllEscape(t *testing.T) {

	tests := []struct {
		input  string
		expect string
	}{
		{
			input:  "\033[1;31mHello, \033[4mworld!\033[0m",
			expect: "Hello, world!",
		},
	}

	for _, tt := range tests {
		output := removeAllEscape(tt.input)
		if output != tt.expect {
			t.Errorf("not matched, actual: %s, expect: %s", output, tt.expect)
		}
	}
}
