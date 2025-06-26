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

package backend

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/pkg/errors"
)

func TestS3Backend_HCL(t *testing.T) {
	type fields struct {
		Region string
		Key    string
		Bucket string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "normal",
			fields: fields{
				Region: "a",
				Key:    "b",
				Bucket: "c",
			},
			want: `
terraform {
  backend s3 {
    bucket = "c"
    key    = "b"
    region = "a"
  }
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := S3Backend{
				Region: tt.fields.Region,
				Key:    tt.fields.Key,
				Bucket: tt.fields.Bucket,
			}
			if got := s.HCL(); got != tt.want {
				t.Errorf("HCL() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockNoSuchBucketError struct {
}

func (err mockNoSuchBucketError) Error() string {
	return ""
}

func (err mockNoSuchBucketError) Message() string {
	return ""
}

func (err mockNoSuchBucketError) OrigErr() error {
	return errors.New(s3.ErrCodeNoSuchBucket)
}

func (err mockNoSuchBucketError) Code() string {
	return s3.ErrCodeNoSuchBucket
}

type mockNoSuchKeyError struct {
}

func (err mockNoSuchKeyError) Error() string {
	return ""
}

func (err mockNoSuchKeyError) Message() string {
	return ""
}

func (err mockNoSuchKeyError) OrigErr() error {
	return errors.New(s3.ErrCodeNoSuchKey)
}

func (err mockNoSuchKeyError) Code() string {
	return s3.ErrCodeNoSuchKey
}

type mockS3Client struct {
	s3iface.S3API
}

func (s *mockS3Client) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	var resp string
	switch {
	case *(input.Bucket) == "a" && *(input.Key) == "a":
		resp = "test_a"

	case *(input.Bucket) == "a" && *(input.Key) == "c":
		return nil, mockNoSuchKeyError{}

	case *(input.Bucket) == "b":
		return nil, mockNoSuchBucketError{}
	}

	if resp != "" {
		body := io.NopCloser(bytes.NewBuffer([]byte(resp)))
		return &s3.GetObjectOutput{Body: body}, nil
	}
	return nil, nil
}

func (s *mockS3Client) DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	if *(input.Bucket) == "a" && *(input.Key) == "a" {
		return &s3.DeleteObjectOutput{}, nil
	}
	return nil, nil
}

func TestS3Backend_GetTFStateJSON(t *testing.T) {
	type fields struct {
		Key    string
		Bucket string
	}
	type args struct {
		in0 context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "bucket exists, key exists",
			fields: fields{
				Key:    "a",
				Bucket: "a",
			},
			want: []byte("test_a"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &S3Backend{
				client: &mockS3Client{},
				Key:    tt.fields.Key,
				Bucket: tt.fields.Bucket,
			}
			got, err := s.GetTFStateJSON(tt.args.in0)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTFStateJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetTFStateJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestS3Backend_CleanUp(t *testing.T) {
	type fields struct {
		Key    string
		Bucket string
	}
	type args struct {
		in0 context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "no such bucket",
			fields: fields{
				Key:    "a",
				Bucket: "b",
			},
		},
		{
			name: "no such key",
			fields: fields{
				Key:    "c",
				Bucket: "a",
			},
		},
		{
			name: "bucket exists, key exists",
			fields: fields{
				Key:    "a",
				Bucket: "a",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &S3Backend{
				client: &mockS3Client{},
				Key:    tt.fields.Key,
				Bucket: tt.fields.Bucket,
			}
			if err := s.CleanUp(tt.args.in0); (err != nil) != tt.wantErr {
				t.Errorf("CleanUp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
