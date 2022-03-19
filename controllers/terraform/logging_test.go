package terraform

import (
	"context"
	"github.com/agiledragon/gomonkey/v2"
	"github.com/oam-dev/terraform-controller/api/types"
	"io"
	"io/ioutil"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	"k8s.io/client-go/rest"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakeclient "k8s.io/client-go/kubernetes/fake"
)

func TestGetPodLog(t *testing.T) {
	ctx := context.Background()

	type prepare func(t *testing.T)
	k8sClientSet := fakeclient.NewSimpleClientset()

	type args struct {
		client            kubernetes.Interface
		namespace         string
		name              string
		containerName     string
		initContainerName string
		prepare
	}
	type want struct {
		state  types.Stage
		log    string
		errMsg string
	}

	p := gomonkey.ApplyMethod(reflect.TypeOf(&http.Client{}), "Do",
		func(_ *http.Client, req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader("xxx")),
			}, nil
		})

	patches := gomonkey.ApplyMethod(reflect.TypeOf(&fake.FakePods{}), "GetLogs",
		func(_ *fake.FakePods, _ string, _ *v1.PodLogOptions) *rest.Request {
			// rate := flowcontrol.NewFakeNeverRateLimiter()
			restClient, _ := rest.NewRESTClient(
				&url.URL{
					Scheme: "http",
					Host:   "127.0.0.1",
				},
				"",
				rest.ClientContentConfig{},
				nil,
				http.DefaultClient)
			r := rest.NewRequest(restClient)
			r.Body([]byte("xxx"))
			return r
		})

	defer p.Reset()
	defer patches.Reset()

	var testcases = []struct {
		name string
		args args
		want want
	}{
		{
			name: "Pod is available, but no logs",
			args: args{
				client:            k8sClientSet,
				namespace:         "default",
				name:              "j1",
				containerName:     "terraform-executor",
				initContainerName: "terraform-init",
				prepare: func(t *testing.T) {
					pod := &v1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "p1",
							Namespace: "default",
							Labels: map[string]string{
								"job-name": "j1",
							},
						},
						TypeMeta: metav1.TypeMeta{
							Kind: "Pod",
						},
						Status: v1.PodStatus{
							Phase: v1.PodPending,
							InitContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "terraform-init",
									Ready: false,
								},
							},
						},
					}
					k8sClientSet.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
				},
			},
			want: want{
				state: types.TerraformInit,
				log:   "xxx",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tc.args.prepare(t)
			state, got, err := getPodLog(ctx, tc.args.client, tc.args.namespace, tc.args.name, tc.args.containerName, tc.args.initContainerName)
			if tc.want.errMsg != "" || err != nil {
				assert.EqualError(t, err, tc.want.errMsg)
			} else {
				assert.Equal(t, tc.want.log, got)
				assert.Equal(t, tc.want.state, state)
			}
		})
	}
}

func TestFlushStream(t *testing.T) {
	type args struct {
		rc   io.ReadCloser
		name string
	}
	type want struct {
		errMsg string
	}

	var testcases = []struct {
		name string
		args args
		want want
	}{
		{
			name: "Flush stream",
			args: args{
				rc:   ioutil.NopCloser(strings.NewReader("xxx")),
				name: "p1",
			},
			want: want{},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := flushStream(tc.args.rc, tc.args.name)
			if tc.want.errMsg != "" {
				assert.Contains(t, err.Error(), tc.want.errMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "xxx", logs)
			}
		})
	}
}
