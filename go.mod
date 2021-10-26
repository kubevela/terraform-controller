module github.com/oam-dev/terraform-controller

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/ghodss/yaml v1.0.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/google/go-cmp v0.5.5
	github.com/onsi/ginkgo v1.16.2
	github.com/onsi/gomega v1.12.0
	github.com/pkg/errors v0.9.1
	google.golang.org/appengine v1.6.5 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v0.18.8
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29 // indirect
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/yaml v1.2.0
)
