module github.com/oam-dev/terraform-controller

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/agiledragon/gomonkey/v2 v2.4.0
	github.com/aliyun/alibaba-cloud-sdk-go v1.61.1384
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/pkg/errors v0.9.1
	github.com/smartystreets/assertions v1.1.0 // indirect
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/net v0.0.0-20210428140749-89ef3d95e781
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/klog/v2 v2.8.0
	sigs.k8s.io/controller-runtime v0.9.5
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/jmespath/go-jmespath v0.0.0-20180206201540-c2b33e8439af => github.com/cloud-native-application/go-jmespath v0.5.0
