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
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/oam-dev/terraform-controller/api/v1beta2"
	"github.com/pkg/errors"
)

const (
	s3AccessKey    = "AWS_ACCESS_KEY_ID"
	s3SecretKey    = "AWS_SECRET_ACCESS_KEY"
	s3SessionToken = "AWS_SESSION_TOKEN"
)

// S3Backend is used to interact with the Terraform s3 backend
type S3Backend struct {
	client       s3iface.S3API
	AccessKey    string
	SecretKey    string
	SessionToken string
	Region       string
	Key          string
	Bucket       string
}

func newS3BackendFromInline(ctx k8sContext, backendConfig *ParsedBackendConfig, optionSource *OptionSource) (Backend, error) {
	region, err := backendConfig.getAttrString("region")
	if err != nil {
		return nil, err
	}
	key, err := backendConfig.getAttrString("key")
	if err != nil {
		return nil, err
	}
	bucket, err := backendConfig.getAttrString("bucket")
	if err != nil {
		return nil, err
	}
	s3Backend := &S3Backend{
		Region: region,
		Key:    key,
		Bucket: bucket,
	}
	if err := s3Backend.fillOptions(ctx, optionSource); err != nil {
		return nil, err
	}
	if err := s3Backend.buildClient(); err != nil {
		return nil, err
	}
	return s3Backend, nil
}

func newS3BackendFromExplicit(ctx k8sContext, backendConfig interface{}, optionSource *OptionSource) (Backend, error) {
	conf, ok := backendConfig.(*v1beta2.S3BackendConf)
	if !ok || conf == nil {
		return nil, errors.New("invalid backendConf")
	}
	s3Backend := &S3Backend{
		Region: conf.Region,
		Key:    conf.Key,
		Bucket: conf.Bucket,
	}
	if err := s3Backend.fillOptions(ctx, optionSource); err != nil {
		return nil, err
	}
	if err := s3Backend.buildClient(); err != nil {
		return nil, err
	}
	return s3Backend, nil
}

func (s *S3Backend) fillOptions(ctx k8sContext, optionSource *OptionSource) error {
	accessKey, ok, err := optionSource.getOption(ctx, s3AccessKey)
	if err != nil || !ok {
		return fmt.Errorf("get option %s error", s3AccessKey)
	}
	s.AccessKey = accessKey

	secretKey, ok, err := optionSource.getOption(ctx, s3SecretKey)
	if err != nil || !ok {
		return fmt.Errorf("get option %s error", s3SecretKey)
	}
	s.SecretKey = secretKey

	token, ok, err := optionSource.getOption(ctx, s3SessionToken)
	if err != nil || !ok {
		s.SessionToken = ""
	}
	s.SessionToken = token

	return nil
}

func (s *S3Backend) buildClient() error {
	sessionOpts := session.Options{
		Config: aws.Config{
			Credentials: credentials.NewStaticCredentials(s.AccessKey, s.SecretKey, s.SessionToken),
			Region:      aws.String(s.Region),
		},
	}
	sess, err := session.NewSessionWithOptions(sessionOpts)
	if err != nil {
		return err
	}
	s.client = s3.New(sess)
	return nil
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
