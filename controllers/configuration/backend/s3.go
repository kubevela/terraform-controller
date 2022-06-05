/*
Copyright 2021 The KubeVela Authors.

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
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awscredentials "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/oam-dev/terraform-controller/controllers/provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// S3Backend is used to interact with the Terraform s3 backend
type S3Backend struct {
	client s3iface.S3API
	Region string
	Key    string
	Bucket string
}

func newS3Backend(_ client.Client, backendConf interface{}, credentials map[string]string) (Backend, error) {
	conf, ok := backendConf.(*v1beta2.S3BackendConf)
	if !ok || conf == nil {
		return nil, fmt.Errorf("invalid backendConf, want *v1beta2.S3BackendConf, but got %#v", backendConf)
	}
	s3Backend := &S3Backend{
		Region: conf.Region,
		Key:    conf.Key,
		Bucket: conf.Bucket,
	}

	accessKey := credentials[provider.EnvAWSAccessKeyID]
	secretKey := credentials[provider.EnvAWSSecretAccessKey]
	sessionToken := credentials[provider.EnvAWSSessionToken]
	if accessKey == "" || secretKey == "" {
		return nil, errors.New("fail to get credentials when build s3 backend")
	}

	// build s3 client
	sessionOpts := session.Options{
		Config: aws.Config{
			Credentials: awscredentials.NewStaticCredentials(accessKey, secretKey, sessionToken),
			Region:      aws.String(s3Backend.Region),
		},
	}
	sess, err := session.NewSessionWithOptions(sessionOpts)
	if err != nil {
		return nil, fmt.Errorf("fial to build s3 backend: %w", err)
	}
	s3Backend.client = s3.New(sess)

	return s3Backend, nil
}

func (s *S3Backend) getObject() (*s3.GetObjectOutput, error) {
	input := &s3.GetObjectInput{
		Key:    &s.Key,
		Bucket: &s.Bucket,
	}
	return s.client.GetObject(input)
}

// GetTFStateJSON gets Terraform state json from the Terraform s3 backend
func (s *S3Backend) GetTFStateJSON(_ context.Context) ([]byte, error) {
	output, err := s.getObject()
	if err != nil {
		return nil, err
	}
	defer func() { _ = output.Body.Close() }()
	writer := bytes.NewBuffer(nil)
	_, err = io.Copy(writer, output.Body)
	if err != nil {
		return nil, err
	}
	return writer.Bytes(), nil
}

// CleanUp will delete the s3 object which contains the Terraform state
func (s *S3Backend) CleanUp(_ context.Context) error {
	_, err := s.getObject()
	if err != nil {
		// nolint:errorlint
		if err, ok := err.(awserr.Error); ok && err.Code() == s3.ErrCodeNoSuchKey || err.Code() == s3.ErrCodeNoSuchBucket {
			// the object is not found, no need to delete
			return nil
		}
		return err
	}

	input := &s3.DeleteObjectInput{
		Bucket: &s.Bucket,
		Key:    &s.Key,
	}
	_, err = s.client.DeleteObject(input)
	return err
}

// HCL returns the backend hcl code string
func (s S3Backend) HCL() string {
	fmtStr := `
terraform {
  backend s3 {
    bucket = "%s"
    key    = "%s"
    region = "%s"
  }
}
`
	return fmt.Sprintf(fmtStr, s.Bucket, s.Key, s.Region)
}
