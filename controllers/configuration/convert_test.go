package configuration

import (
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
	"testing"
)

func TestRawExtension2Map(t *testing.T) {
	type want struct {
		result interface{}
		err    error
	}

	type Spec struct {
		// +kubebuilder:pruning:PreserveUnknownFields
		Variable *runtime.RawExtension `json:"variable,omitempty"`
	}

	cases := map[string]struct {
		variable string
		want     want
	}{
		"StringType": {
			variable: `
Variable:
  k: Will
`,
			want: want{
				result: "Will",
				err:    nil,
			},
		},
		"ListType1": {
			variable: `
Variable:
  k: ["Will", "Catherine"]
`,
			want: want{
				result: []interface{}{"Will", "Catherine"},
				err:    nil,
			},
		},
		"ListType2": {
			variable: `
Variable:
  k:
    - "Will"
    - "Catherine"
`,
			want: want{
				result: []interface{}{"Will", "Catherine"},
				err:    nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var spec Spec
			err := yaml.Unmarshal([]byte(tc.variable), &spec)
			assert.NilError(t, err)
			result, err := RawExtension2Map(spec.Variable)
			assert.Equal(t, tc.want.err, err)
			assert.DeepEqual(t, tc.want.result, result["k"])
		})
	}
}

func TestInterface2String(t *testing.T) {
	type want struct {
		result string
		err    error
	}
	cases := map[string]struct {
		variable interface{}
		want     want
	}{
		"StringType": {
			variable: "Will",
			want: want{
				result: "Will",
				err:    nil,
			},
		},
		"IntType": {
			variable: 123,
			want: want{
				result: "123",
				err:    nil,
			},
		},
		"BoolType": {
			variable: true,
			want: want{
				result: "true",
				err:    nil,
			},
		},
		"ListType1": {
			variable: []interface{}{"Will", "Catherine"},
			want: want{
				result: "'[\"Will\", \"Catherine\", ]'",
				err:    nil,
			},
		},
		"ListType2": {
			variable: []interface{}{123, 456},
			want: want{
				result: "'[123, 456, ]'",
				err:    nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Interface2String(tc.variable)
			assert.Equal(t, tc.want.err, err)
			assert.DeepEqual(t, tc.want.result, result)
		})
	}
}
