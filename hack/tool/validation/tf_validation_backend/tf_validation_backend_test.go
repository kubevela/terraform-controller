/*
Copyright 2022 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
)

func Test_fetchBackendName(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		want    string
		wantErr bool
	}{
		{
			name: "valid backend block",
			hcl: `
			  terraform {
				backend "remote" {
				  workspace {
					prefix = "test_"
				  }
				}
			  }
			`,
			want:    "remote",
			wantErr: false,
		},
		{
			name: "empty backend block",
			hcl: `
			  terraform {
				backend "remote" {
				}
			  }
			`,
			want:    "remote",
			wantErr: false,
		},
		{
			name: "invalid backend block",
			hcl: `
			  terraform {
				backend {
				  workspace {
					prefix = "test_"
				  }
				}
			  }
			`,
			want:    "",
			wantErr: true,
		},
		{
			name: "no backend block",
			hcl: `
			  terraform {
			  }
			`,
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, _ := hclparse.NewParser().ParseHCL([]byte(tt.hcl), "test.tf")
			got, err := fetchBackendName(file.Body)
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchBackendName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("fetchBackendName() = %v, want %v", got, tt.want)
			}
		})
	}
}
