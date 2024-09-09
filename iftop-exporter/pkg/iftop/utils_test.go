package iftop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractIP(t *testing.T) {
	tests := []struct {
		addr   string
		expect string
	}{
		{
			addr:   "1.2.3.4",
			expect: "1.2.3.4",
		},
		{
			addr:   "1.2.3.4:5678",
			expect: "1.2.3.4",
		},
		{
			addr:   "::FFFF:C0A8:1%1",
			expect: "::FFFF:C0A8:1%1",
		},
		{
			addr:   "[::FFFF:C0A8:1%1]:80",
			expect: "::FFFF:C0A8:1%1",
		},
	}

	for _, tt := range tests {
		actual := extractIP(tt.addr)
		assert.Equal(t, tt.expect, actual)
	}
}
