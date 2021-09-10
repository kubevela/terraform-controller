package configuration

import (
	"os"
	"strings"
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

func TestCheckTFConfiguration(t *testing.T) {
	cases := map[string]struct {
		name          string
		configuration string
		subStr        string
	}{
		"Invalid": {
			name: "bad",
			configuration: `resource2 "alicloud_oss_bucket" "bucket-acl" {
  bucket = var.bucket
  acl = var.acl
}

output "BUCKET_NAME" {
  value = "${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}"
}

variable "bucket" {
  description = "OSS bucket name"
  default = "vela-website"
  type = string
}

variable "acl" {
  description = "OSS bucket ACL, supported 'private', 'public-read', 'public-read-write'"
  default = "private"
  type = string
}
`,
			subStr: "Error:",
		},
		"valid": {
			name: "good",
			configuration: `resource "alicloud_oss_bucket" "bucket-acl" {
  bucket = var.bucket
  acl = var.acl
}

output "BUCKET_NAME" {
  value = "${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}"
}

variable "bucket" {
  description = "OSS bucket name"
  default = "vela-website"
  type = string
}

variable "acl" {
  description = "OSS bucket ACL, supported 'private', 'public-read', 'public-read-write'"
  default = "private"
  type = string
}`,
			subStr: "",
		},
	}
	// As the entry point is the root folder `terraform-controller`, the unit-test locates here `./controllers/configuration`,
	// so we change the directory
	os.Chdir("../../")
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := checkTerraformSyntax(tc.name, tc.configuration)
			if err != nil {
				if !strings.Contains(err.Error(), tc.subStr) {
					t.Errorf("\ncheckTFConfiguration(...) %s\n", cmp.Diff(err.Error(), tc.subStr))
				}
			}
		})
	}
}
