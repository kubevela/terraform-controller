package util

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
)

func TestCompareTwoContainerEnvs(t *testing.T) {
	cases := map[string]struct {
		s1   []v1.EnvVar
		s2   []v1.EnvVar
		want bool
	}{
		"Equal": {
			s1: []v1.EnvVar{
				{
					Name:  "TF_VAR_zone_id",
					Value: "cn-beijing-z",
				},
				{
					Name:  "E1",
					Value: "V1",
				},
				{
					Name:  "NAME",
					Value: "aaa",
				},
			},
			s2: []v1.EnvVar{

				{
					Name:  "NAME",
					Value: "aaa",
				},
				{
					Name:  "TF_VAR_zone_id",
					Value: "cn-beijing-z",
				},
				{
					Name:  "E1",
					Value: "V1",
				},
			},
			want: true,
		},
		"Not Equal": {
			s1: []v1.EnvVar{
				{
					Name:  "NAME",
					Value: "aaa",
				},
			},
			s2: []v1.EnvVar{

				{
					Name:  "NAME",
					Value: "bbb",
				},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			equal := CompareTwoContainerEnvs(tc.s1, tc.s2)
			if equal != tc.want {
				if diff := cmp.Diff(tc.want, equal); diff != "" {
					t.Errorf("\nCompareTwoContainerEnvs(...) %s\n", diff)
				}
			}
		})
	}
}
