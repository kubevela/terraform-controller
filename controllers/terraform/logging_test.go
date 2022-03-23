package terraform

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"

	"github.com/oam-dev/terraform-controller/api/types"
)

func TestGetPodLog(t *testing.T) {
	ctx := context.Background()
	type args struct {
		client            kubernetes.Interface
		namespace         string
		name              string
		containerName     string
		initContainerName string
	}
	type want struct {
		state  types.Stage
		log    string
		errMsg string
	}

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
			Phase: v1.PodRunning,
		},
	}

	k8sClientSet := fakeclient.NewSimpleClientset(pod)

	patches := gomonkey.ApplyMethod(reflect.TypeOf(&fake.FakePods{}), "GetLogs",
		func(_ *fake.FakePods, _ string, _ *v1.PodLogOptions) *rest.Request {
			rate := flowcontrol.NewFakeNeverRateLimiter()
			restClient, _ := rest.NewRESTClient(
				&url.URL{
					Scheme: "http",
					Host:   "",
				},
				"",
				rest.ClientContentConfig{},
				rate,
				http.DefaultClient)
			r := rest.NewRequest(restClient)
			r.Body([]byte("xxx"))
			return r
		})
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
			},
			want: want{
				errMsg: "can not be accept",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
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
