package configuration

import (
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func TestRawExtension2Map(t *testing.T) {
	type want struct {
		result interface{}
		err    error
	}

	type Spec struct {
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
		"nil": {
			want: want{
				result: nil,
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
			if tc.want.err != nil {
				assert.Error(t, err, tc.want.err.Error())
			} else {
				assert.Equal(t, tc.want.err, err)
				assert.DeepEqual(t, tc.want.result, result["k"])
			}
		})
	}
}

func TestRawExtension2Map2(t *testing.T) {
	type args struct {
		raw *runtime.RawExtension
	}
	type want struct {
		errMessage string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"bad raw": {
			args: args{
				raw: &runtime.RawExtension{
					Raw: []byte("xxx"),
				},
			},
			want: want{
				errMessage: "invalid character 'x' looking for beginning of value",
			},
		}}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := RawExtension2Map(tc.args.raw)
			if tc.want.errMessage != "" {
				assert.Error(t, err, tc.want.errMessage)
			} else {
				assert.NilError(t, err)
			}
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
		"NumberType": {
			variable: 1024.1,
			want: want{
				result: "1024.1",
				err:    nil,
			},
		},
		"ListType1": {
			variable: []interface{}{"Will", "Catherine"},
			want: want{
				result: `["Will","Catherine"]`,
				err:    nil,
			},
		},
		"ListType2": {
			variable: []interface{}{123, 456},
			want: want{
				result: `[123,456]`,
				err:    nil,
			},
		},
		"ObjectType": {
			variable: struct{ Name string }{"Terraform"},
			want: want{
				result: `{"Name":"Terraform"}`,
				err:    nil,
			},
		},
		"ListObjectType": {
			variable: []struct{ Name string }{{"Terraform"}, {"OAM"}, {"Vela"}},
			want: want{
				result: `[{"Name":"Terraform"},{"Name":"OAM"},{"Name":"Vela"}]`,
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
