package cogs

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func TestKeys(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "Basic",
			input:  `host: auth.knoxnetworks.com`,
			output: `host: auth.knoxnetworks.com`,
		},
		{
			name:   "GoTemplate",
			input:  `serviceName: {{ include "auth-mgmt.fullname" . }}`,
			output: `serviceName: {{ include "auth-mgmt.fullname" . }}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootNode := &yaml.Node{}
			if err := yaml.Unmarshal([]byte(tc.input), rootNode); err != nil {
				t.Error(errors.Wrap(err, tc.name))
			}
			node := &Node{inner: rootNode}
			node.GoTemplateToStr()

			b, err := yaml.Marshal(node.inner)
			if err != nil {
				t.Error(errors.Wrap(err, tc.name))
			}
			actual := StripGoTemplateDelim(string(b))

			expected := fmt.Sprintf("%s\n", tc.output)
			if diff := cmp.Diff(expected, string(actual)); diff != "" {
				t.Errorf("[%s]: (-expected err +actual err)\n-%s", tc.name, diff)
			}
		})

	}
}
