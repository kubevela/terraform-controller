package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/go-yaml/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	types "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	"github.com/oam-dev/terraform-controller/api/v1beta1"
)

func TestCheckAlibabaCloudCredentials(t *testing.T) {
	type credentials struct {
		AccessKeyID     string
		AccessKeySecret string
		SecurityToken   string
		Region          string
	}

	type args struct {
		credentials credentials
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Check AlibabaCloud credentials",
			args: args{
				credentials: credentials{
					AccessKeyID:     "aaaa",
					AccessKeySecret: "bbbbb",
					Region:          "cn-hangzhou",
				},
			},
		},
		{
			name: "Check AlibabaCloud credentials with sts token",
			args: args{
				credentials: credentials{
					AccessKeyID:     "STS.aaaa",
					AccessKeySecret: "bbbbb",
					SecurityToken:   "ccc",
					Region:          "cn-hangzhou",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := tt.args.credentials
			err := checkAlibabaCloudCredentials(cred.Region, cred.AccessKeyID, cred.AccessKeySecret, cred.SecurityToken)
			assert.NotNil(t, err)
		})
	}
}

func newFakeClient4CustomProvider() client.Client {
	objects := []runtime.Object{
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-custom",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"credentials": []byte("Token: mytoken"),
			},
			Type: v1.SecretTypeOpaque,
		},
	}

	return fake.NewFakeClient(objects...)
}

func TestGetProviderCredentials(t *testing.T) {
	ctx := context.TODO()
	k8sClient1 := fake.NewClientBuilder().Build()

	ak := AlibabaCloudCredentials{
		AccessKeyID:     "aaaa",
		AccessKeySecret: "bbbbb",
	}
	credentials, err := yaml.Marshal(&ak)
	assert.Nil(t, err)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": credentials,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient1.Create(ctx, secret))

	patches := ApplyMethod(reflect.TypeOf(&sts.Client{}), "GetCallerIdentity", func(_ *sts.Client, request *sts.GetCallerIdentityRequest) (response *sts.GetCallerIdentityResponse, err error) {
		response = nil
		err = nil
		return
	})
	defer patches.Reset()

	defaultProvider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: "alibaba",
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	// secret's key is wrong
	k8sClient2 := fake.NewClientBuilder().Build()
	secret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient2.Create(ctx, secret))

	// baidu
	k8sClient4Baidu := fake.NewClientBuilder().Build()
	baiduCredentials, _ := yaml.Marshal(&BaiduCloudCredentials{
		KeyBaiduAccessKey: "aaaa",
		KeyBaiduSecretKey: "bbbbb",
	})
	secret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": baiduCredentials,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient4Baidu.Create(ctx, secret))

	var baiduProvider v1beta1.Provider
	copier.Copy(&baiduProvider, &defaultProvider)

	baiduProvider.Spec.Provider = string(baidu)

	// ec
	k8sClient4EC := fake.NewClientBuilder().Build()
	ecCredentials, _ := yaml.Marshal(&ECCredentials{
		ECApiKey: "aaaa",
	})
	secret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": ecCredentials,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient4EC.Create(ctx, secret))

	var ecProvider v1beta1.Provider
	copier.Copy(&ecProvider, &defaultProvider)

	ecProvider.Spec.Provider = string(ec)

	// not supported provider
	var notSupportedProvider v1beta1.Provider
	copier.Copy(&notSupportedProvider, &defaultProvider)
	notSupportedProvider.Spec.Provider = "abc"

	// alibaba provider with wrong data
	secret3 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient3 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient3.Create(ctx, secret3))

	var alibabaProvider v1beta1.Provider
	copier.CopyWithOption(&alibabaProvider, &defaultProvider, copier.Option{DeepCopy: true})
	alibabaProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	// baidu cloud provider with wrong data
	var baiduProvider2 v1beta1.Provider
	copier.CopyWithOption(&baiduProvider2, &defaultProvider, copier.Option{DeepCopy: true})
	baiduProvider2.Spec.Provider = string(baidu)
	baiduProvider2.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "Other source",
			args: args{
				k8sClient: k8sClient1,
				provider: v1beta1.Provider{
					Spec: v1beta1.ProviderSpec{
						Credentials: v1beta1.ProviderCredentials{
							Source: "Nil",
						},
					},
				},
			},
			errMsg: "the credentials type is not supported.",
		},
		{
			name: "Secret not found",
			args: args{
				k8sClient: k8sClient1,
				provider: v1beta1.Provider{
					Spec: v1beta1.ProviderSpec{
						Credentials: v1beta1.ProviderCredentials{
							Source: "Secret",
							SecretRef: &types.SecretKeySelector{
								SecretReference: types.SecretReference{
									Name:      "nil",
									Namespace: "default",
								},
								Key: "credentials",
							},
						},
					},
				},
			},
			errMsg: "failed to get the Secret from Provider",
		},
		{
			name: "Secret found with wrong key",
			args: args{
				k8sClient: k8sClient2,
				provider: v1beta1.Provider{
					Spec: v1beta1.ProviderSpec{
						Provider: "alibaba",
						Credentials: v1beta1.ProviderCredentials{
							Source: "Secret",
							SecretRef: &types.SecretKeySelector{
								SecretReference: types.SecretReference{
									Name:      "default",
									Namespace: "default",
								},
								Key: "credentials",
							},
						},
					},
				},
				region: "xxx",
			},
			errMsg: "not found in the referenced secret",
		},
		{
			name: "Secret found with right key",
			args: args{
				k8sClient: k8sClient1,
				provider:  defaultProvider,
				region:    "xxx",
			},
			want: map[string]string{
				envAlicloudAcessKey:  ak.AccessKeyID,
				envAlicloudSecretKey: ak.AccessKeySecret,
				envAlicloudRegion:    "xxx",
				envAliCloudStsToken:  ak.SecurityToken,
			},
		},
		{
			name: "alibaba provider with wrong data",
			args: args{
				k8sClient: k8sClient3,
				provider:  alibabaProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
		{
			name: "baidu provider with wrong data",
			args: args{
				k8sClient: k8sClient3,
				provider:  baiduProvider2,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
		{
			name: "Custom Provider",
			args: args{
				k8sClient: newFakeClient4CustomProvider(),
				provider: v1beta1.Provider{
					Spec: v1beta1.ProviderSpec{
						Provider: string(custom),
						Credentials: v1beta1.ProviderCredentials{
							Source: "Secret",
							SecretRef: &types.SecretKeySelector{
								SecretReference: types.SecretReference{
									Name:      "test-custom",
									Namespace: "default",
								},
								Key: "credentials",
							},
						},
					},
				},
			},
			want: map[string]string{
				"Token": "mytoken",
			},
		},
		{
			name: "baidu cloud",
			args: args{
				k8sClient: k8sClient4Baidu,
				provider:  baiduProvider,
				region:    "xxx",
			},
			want: map[string]string{
				envBaiduAccessKey: "aaaa",
				envBaiduSecretKey: "bbbbb",
				envBaiduRegion:    "xxx",
			},
		},
		{
			name: "EC",
			args: args{
				k8sClient: k8sClient4EC,
				provider:  ecProvider,
			},
			want: map[string]string{
				envECApiKey: "aaaa",
			},
		},
		{
			name: "not supported provider",
			args: args{
				k8sClient: k8sClient1,
				provider:  notSupportedProvider,
				region:    "xxx",
			},
			errMsg: "unsupported provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProviderCredentials4EC(t *testing.T) {
	ctx := context.TODO()
	k8sClient4EC := fake.NewClientBuilder().Build()
	ecCredentials, _ := yaml.Marshal(&ECCredentials{
		ECApiKey: "aaaa",
	})
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": ecCredentials,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient4EC.Create(ctx, secret))

	provider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: string(ec),
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient2.Create(ctx, secret2))

	var badProvider v1beta1.Provider
	copier.CopyWithOption(&badProvider, &provider, copier.Option{DeepCopy: true})
	badProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "EC",
			args: args{
				k8sClient: k8sClient4EC,
				provider:  provider,
			},
			want: map[string]string{
				envECApiKey: "aaaa",
			},
		},
		{
			name: "provider with wrong data",
			args: args{
				k8sClient: k8sClient2,
				provider:  badProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProviderCredentials4VSphere(t *testing.T) {
	ctx := context.TODO()
	k8sClient := fake.NewClientBuilder().Build()
	creds, _ := yaml.Marshal(&VSphereCredentials{
		VSphereUser:               "a",
		VSpherePassword:           "b",
		VSphereServer:             "c",
		VSphereAllowUnverifiedSSL: "d",
	})
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": creds,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient.Create(ctx, secret))

	provider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: string(vsphere),
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient2.Create(ctx, secret2))

	var badProvider v1beta1.Provider
	copier.CopyWithOption(&badProvider, &provider, copier.Option{DeepCopy: true})
	badProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "provider",
			args: args{
				k8sClient: k8sClient,
				provider:  provider,
			},
			want: map[string]string{
				envVSphereUser:               "a",
				envVSpherePassword:           "b",
				envVSphereServer:             "c",
				envVSphereAllowUnverifiedSSL: "d",
			},
		},
		{
			name: "provider with wrong data",
			args: args{
				k8sClient: k8sClient2,
				provider:  badProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProviderCredentials4TencentCloud(t *testing.T) {
	ctx := context.TODO()
	k8sClient := fake.NewClientBuilder().Build()
	creds, _ := yaml.Marshal(&TencentCloudCredentials{
		SecretID:  "a",
		SecretKey: "b",
	})
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": creds,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient.Create(ctx, secret))

	provider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: string(tencent),
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient2.Create(ctx, secret2))

	var badProvider v1beta1.Provider
	copier.CopyWithOption(&badProvider, &provider, copier.Option{DeepCopy: true})
	badProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "provider",
			args: args{
				k8sClient: k8sClient,
				provider:  provider,
				region:    "bj",
			},
			want: map[string]string{
				envQCloudSecretID:  "a",
				envQCloudSecretKey: "b",
				envQCloudRegion:    "bj",
			},
		},
		{
			name: "provider with wrong data",
			args: args{
				k8sClient: k8sClient2,
				provider:  badProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProviderFromConfiguration(t *testing.T) {
	ctx := context.Background()
	k8sClient1 := fake.NewClientBuilder().Build()

	s := runtime.NewScheme()
	v1beta1.AddToScheme(s)
	provider := &v1beta1.Provider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "terraform.core.oam.dev/v1beta1",
			Kind:       "Provider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "a",
		},
	}
	k8sClient2 := fake.NewClientBuilder().WithScheme(s).WithObjects(provider).Build()

	type args struct {
		k8sClient client.Client
		namespace string
		name      string
	}
	type want struct {
		provider *v1beta1.Provider
		errMsg   string
	}
	testcases := []struct {
		name string
		args args
		want want
	}{
		{
			name: "failed to get provider",
			args: args{
				k8sClient: k8sClient1,
				namespace: "a",
				name:      "b",
			},
			want: want{
				errMsg: "failed to get Provider object",
			},
		},
		{
			name: "provider is not found",
			args: args{
				k8sClient: k8sClient2,
				namespace: "a",
				name:      "b",
			},
			want: want{},
		},
		{
			name: "provider is found",
			args: args{
				k8sClient: k8sClient2,
				namespace: "a",
				name:      "a",
			},
			want: want{
				provider: provider,
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetProviderFromConfiguration(ctx, tc.args.k8sClient, tc.args.namespace, tc.args.name)
			if err != nil {
				if !strings.Contains(err.Error(), tc.want.errMsg) {
					t.Errorf("IsDeletable() error = %v, wantErr %v", err, tc.want.errMsg)
					return
				}
			}
			if tc.want.provider != nil && !reflect.DeepEqual(got, tc.want.provider) {
				t.Errorf("IsDeletable() differs between got and want: %s", cmp.Diff(got, tc.want.provider))
			}
		})
	}
}

func TestGetProviderCredentials4UCloud(t *testing.T) {
	ctx := context.TODO()
	k8sClient := fake.NewClientBuilder().Build()
	creds, _ := yaml.Marshal(&UCloudCredentials{
		PublicKey:  "a",
		PrivateKey: "b",
		Region:     "bj",
		ProjectID:  "c",
	})
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": creds,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient.Create(ctx, secret))

	provider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: string(ucloud),
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient2.Create(ctx, secret2))

	var badProvider v1beta1.Provider
	copier.CopyWithOption(&badProvider, &provider, copier.Option{DeepCopy: true})
	badProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "provider",
			args: args{
				k8sClient: k8sClient,
				provider:  provider,
				region:    "bj",
			},
			want: map[string]string{
				envUCloudPublicKey:  "a",
				envUCloudPrivateKey: "b",
				envUCloudRegion:     "bj",
				envUCloudProjectID:  "c",
			},
		},
		{
			name: "provider with wrong data",
			args: args{
				k8sClient: k8sClient2,
				provider:  badProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProviderCredentials4GCP(t *testing.T) {
	ctx := context.TODO()
	k8sClient := fake.NewClientBuilder().Build()
	creds, _ := yaml.Marshal(&GCPCredentials{
		GCPCredentialsJSON: "a",
		GCPProject:         "b",
	})
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": creds,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient.Create(ctx, secret))

	provider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: string(gcp),
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient2.Create(ctx, secret2))

	var badProvider v1beta1.Provider
	copier.CopyWithOption(&badProvider, &provider, copier.Option{DeepCopy: true})
	badProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "provider",
			args: args{
				k8sClient: k8sClient,
				provider:  provider,
				region:    "bj",
			},
			want: map[string]string{
				envGCPCredentialsJSON: "a",
				envGCPProject:         "b",
				envGCPRegion:          "bj",
			},
		},
		{
			name: "provider with wrong data",
			args: args{
				k8sClient: k8sClient2,
				provider:  badProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProviderCredentials4AWS(t *testing.T) {
	ctx := context.TODO()
	k8sClient := fake.NewClientBuilder().Build()
	creds, _ := yaml.Marshal(&AWSCredentials{
		AWSAccessKeyID:     "a",
		AWSSecretAccessKey: "b",
		AWSSessionToken:    "c",
	})
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": creds,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient.Create(ctx, secret))

	provider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: string(aws),
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient2.Create(ctx, secret2))

	var badProvider v1beta1.Provider
	copier.CopyWithOption(&badProvider, &provider, copier.Option{DeepCopy: true})
	badProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "provider",
			args: args{
				k8sClient: k8sClient,
				provider:  provider,
				region:    "bj",
			},
			want: map[string]string{
				envAWSAccessKeyID:     "a",
				envAWSSecretAccessKey: "b",
				envAWSSessionToken:    "c",
				envAWSDefaultRegion:   "bj",
			},
		},
		{
			name: "provider with wrong data",
			args: args{
				k8sClient: k8sClient2,
				provider:  badProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProviderCredentials4Azure(t *testing.T) {
	ctx := context.TODO()
	k8sClient := fake.NewClientBuilder().Build()
	creds, _ := yaml.Marshal(&AzureCredentials{
		ARMClientID:       "a",
		ARMClientSecret:   "b",
		ARMSubscriptionID: "c",
		ARMTenantID:       "d",
	})
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": creds,
		},
		Type: v1.SecretTypeOpaque,
	}
	assert.Nil(t, k8sClient.Create(ctx, secret))

	provider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: string(azure),
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient2.Create(ctx, secret2))

	var badProvider v1beta1.Provider
	copier.CopyWithOption(&badProvider, &provider, copier.Option{DeepCopy: true})
	badProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "provider",
			args: args{
				k8sClient: k8sClient,
				provider:  provider,
				region:    "bj",
			},
			want: map[string]string{
				envARMClientID:       "a",
				envARMClientSecret:   "b",
				envARMSubscriptionID: "c",
				envARMTenantID:       "d",
			},
		},
		{
			name: "provider with wrong data",
			args: args{
				k8sClient: k8sClient2,
				provider:  badProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetProviderCredentials4Custom(t *testing.T) {
	ctx := context.TODO()

	provider := v1beta1.Provider{
		Spec: v1beta1.ProviderSpec{
			Provider: string(azure),
			Credentials: v1beta1.ProviderCredentials{
				Source: "Secret",
				SecretRef: &types.SecretKeySelector{
					SecretReference: types.SecretReference{
						Name:      "default",
						Namespace: "default",
					},
					Key: "credentials",
				},
			},
		},
	}

	secret2 := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-data",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("xxx"),
		},
		Type: v1.SecretTypeOpaque,
	}
	k8sClient2 := fake.NewClientBuilder().Build()
	assert.Nil(t, k8sClient2.Create(ctx, secret2))

	var badProvider v1beta1.Provider
	copier.CopyWithOption(&badProvider, &provider, copier.Option{DeepCopy: true})
	badProvider.Spec.Credentials.SecretRef.Name = "wrong-data"

	type args struct {
		provider  v1beta1.Provider
		region    string
		k8sClient client.Client
	}
	tests := []struct {
		name   string
		args   args
		want   map[string]string
		errMsg string
	}{
		{
			name: "provider with wrong data",
			args: args{
				k8sClient: k8sClient2,
				provider:  badProvider,
				region:    "xxx",
			},
			errMsg: errConvertCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, tt.args.k8sClient, &tt.args.provider, tt.args.region)
			if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetProviderCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}
