package utils

import (
	"fmt"
	"strings"
)

// Selectors is a list of Selector, and the items are OR-ed when used.
type Selectors []Selector

type Selector struct {
	Name  string
	KopVs []KopV
}

// KopV defines how to check labelKey/labelValue
//
//   - labelKey != labelValue
//   - labelKey == labelValue
//   - labelKey                 (key exists)
type KopV struct {
	Key   string
	Op    string // ONLY "==" and "!=" are supported.
	Value string
}

func (ss Selectors) Hit(podLabels map[string]string) bool {
	if len(ss) == 0 {
		// no selectors means all pods are not matched.
		return false
	}

	for _, selector := range ss {
		if len(selector.KopVs) == 0 {
			// empty KopVs means not matched, continue to match the next selector.
			continue
		}

		hit := true
	S:
		for _, kopv := range selector.KopVs {
			labelKey := kopv.Key
			labelOp := kopv.Op
			labelValue := kopv.Value

			v, exists := podLabels[labelKey]
			if !exists {
				hit = false
				break S
			}

			switch labelOp {
			case "":
				continue
			case "!=":
				if v == labelValue {
					hit = false
					break S
				}
			case "==":
				if v != labelValue {
					hit = false
					break S
				}
			}
		}

		if hit {
			return true
		}
	}

	return false
}

// Valid formats for input selector: "{SelectorName}:{LabelKeyOperatorValue},{LabelKeyOperatorValue},{LabelKeyOperatorValue}"
//
// Multiple {LabelKeyOperatorValue} can be specified, and they would be AND-ed when used.
//
// Valid formats of {LabelKeyOperatorValue} are:
//
//   - "{LabelKey}{LabelOperator}{LabelValue}"
//   - "{LabelKey}{LabelOperator}"
//   - "{LabelKey}"
//
// Note:
//   - Only "=", "==", "!=" are valid labelOperators. "=" and "==" have same result.
//   - If LabelValue is omitted, it would be set to empty string.
//   - If LabelOperator is omitted, it means to check the existence of LabelKey.
//
// Samples:
//   - name1:some.key.label1/test.com==some-value1,some.key.label2!=some-value2
//   - name2:some.key.label3
func ParseSelector(input string) (*Selector, error) {
	parts := strings.SplitN(input, ":", 2)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty selector")
	}

	name := parts[0]
	kopvs := make([]KopV, 0)

	if len(parts) > 1 {
		for _, kopvStr := range strings.Split(parts[1], ",") {
			if strings.Contains(kopvStr, "!=") {
				fields := strings.Split(kopvStr, "!=")
				kopvs = append(kopvs, KopV{
					Key:   fields[0],
					Op:    "!=",
					Value: fields[1],
				})
			} else if strings.Contains(kopvStr, "==") {
				fields := strings.Split(kopvStr, "==")
				kopvs = append(kopvs, KopV{
					Key:   fields[0],
					Op:    "==",
					Value: fields[1],
				})
			} else if strings.Contains(kopvStr, "=") {
				fields := strings.Split(kopvStr, "=")
				kopvs = append(kopvs, KopV{
					Key:   fields[0],
					Op:    "==",
					Value: fields[1],
				})
			} else {
				kopvs = append(kopvs, KopV{
					Key:   kopvStr,
					Op:    "",
					Value: "",
				})
			}
		}
	}

	return &Selector{
		Name:  name,
		KopVs: kopvs,
	}, nil
}

func ParseSelectors(inputs []string) (Selectors, error) {
	ret := Selectors{}

	for _, input := range inputs {
		selector, err := ParseSelector(input)
		if err != nil {
			return ret, err
		}
		ret = append(ret, *selector)
	}

	return ret, nil
}
