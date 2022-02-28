package terraform

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
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
)

func TestGetPodLog(t *testing.T) {
	ctx := context.Background()
	type args struct {
		client        kubernetes.Interface
		namespace     string
		name          string
		containerName string
	}
	type want struct {
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
				client:        k8sClientSet,
				namespace:     "default",
				name:          "j1",
				containerName: "terraform-executor",
			},
			want: want{
				errMsg: "can not be accept",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getPodLog(ctx, tc.args.client, tc.args.namespace, tc.args.name, tc.args.containerName)
			if tc.want.errMsg != "" {
				assert.EqualError(t, err, tc.want.errMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want.log, got)
			}
		})
	}

}
