package provider

import (
	"context"
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"reflect"
	"strings"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
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

func newFakeClient() client.Client {
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
	client := newFakeClient()

	ak := AlibabaCloudCredentials{
		AccessKeyID:     "aaaa",
		AccessKeySecret: "bbbbb",
	}
	credentials, err := json.Marshal(&ak)
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
	assert.Nil(t, client.Create(ctx, secret))

	patches := ApplyMethod(reflect.TypeOf(&sts.Client{}), "GetCallerIdentity", func(_ *sts.Client, request *sts.GetCallerIdentityRequest) (response *sts.GetCallerIdentityResponse, err error) {
		response = nil
		err = nil
		return
	})
	defer patches.Reset()

	type args struct {
		provider *v1beta1.Provider
		region   string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{
			name: "Other source",
			args: args{
				provider: &v1beta1.Provider{
					Spec: v1beta1.ProviderSpec{
						Credentials: v1beta1.ProviderCredentials{
							Source: "Nil",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Secret not found",
			args: args{
				provider: &v1beta1.Provider{
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
			wantErr: true,
		},
		{
			name: "Secret found",
			args: args{
				provider: &v1beta1.Provider{
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
			want: map[string]string{
				envAlicloudAcessKey:  ak.AccessKeyID,
				envAlicloudSecretKey: ak.AccessKeySecret,
				envAlicloudRegion:    "xxx",
				envAliCloudStsToken:  ak.SecurityToken,
			},
		},
		{
			name: "Custom Provider",
			args: args{
				provider: &v1beta1.Provider{
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProviderCredentials(ctx, client, tt.args.provider, tt.args.region)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProviderCredentials() error = %v, wantErr %v", err, tt.wantErr)
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
