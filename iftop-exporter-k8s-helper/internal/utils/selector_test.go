package utils

import (
	"fmt"
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

func TestSelectors(t *testing.T) {
	var tests = []struct {
		number         int
		inputSelectors []string
		inputLabels    map[string]string
		hitExpected    bool
	}{
		{
			number: 1,
			inputSelectors: []string{
				"selector1:some.key.label1/test.com==some-value1,some.key.label2!=some-value2",
				"selector2:some.key.label3/test.com==some-value3,some.key.label4!=some-value4",
				"selector3:some.key.label3/test.com==",
			},
			inputLabels: map[string]string{
				"some.key.label1/test.com": "some-value1",
				"some.key.label2":          "not-value2",
				"some.key.label3":          "some-value3",
			},
			hitExpected: true,
		},
		{
			number: 2,
			inputSelectors: []string{
				"selector1:some.key.label1/test.com==some-value1,some.key.label2!=some-value2",
				"selector2:some.key.label3/test.com=some-value3,some.key.label4!=some-value4",
				"selector3:some.key.label3/test.com==",
			},
			inputLabels: map[string]string{
				"some.key.label1/test.com": "some-value1",
				"some.key.label2":          "some-value2",
				"some.key.label3":          "some-value3",
			},
			hitExpected: false,
		},
		{
			number: 3,
			inputSelectors: []string{
				"selector1:some.key.label1/test.com=some-value1,some.key.label2!=some-value2",
				"selector2:some.key.label3/test.com==some-value3,some.key.label4!=some-value4",
				"selector3:some.key.label3/test.com==",
			},
			inputLabels: map[string]string{
				"some.key.label1/test.com": "some-value1",
				"some.key.label2":          "some-value2",
				"some.key.label3/test.com": "some-value3",
				"some.key.label4":          "not-value4",
			},
			hitExpected: true,
		},
		{
			number: 4,
			inputSelectors: []string{
				"selector1:some.key.label1/test.com==some-value1,some.key.label2!=some-value2",
				"selector2:some.key.label3/test.com==some-value3,some.key.label4!=some-value4",
				"selector3:some.key.label3/test.com==",
			},
			inputLabels: map[string]string{
				"some.key.label1/test.com": "some-value1",
				"some.key.label2":          "some-value2",
				"some.key.label3/test.com": "",
				"some.key.label4":          "not-value4",
			},
			hitExpected: true,
		},
	}

	for _, tt := range tests {
		selectors, err := ParseSelectors(tt.inputSelectors)
		if err != nil {
			t.Error(err)
		}
		pretty.Println(selectors)

		hitActual := selectors.Hit(tt.inputLabels)
		assert.Equal(t, tt.hitExpected, hitActual, fmt.Sprintf("test #%d hit expect not matched", tt.number))
	}

}
