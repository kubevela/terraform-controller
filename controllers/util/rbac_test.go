package util

import (
	"context"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	env       envtest.Environment
	k8sClient client.Client
)

var _ = ginkgo.BeforeSuite(func() {
	env = envtest.Environment{}
	cfg, err := env.Start()
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(cfg).ToNot(gomega.BeNil())
	k8sClient, err = client.New(cfg, client.Options{})
	gomega.Expect(err).To(gomega.BeNil())
})

var _ = ginkgo.AfterSuite(func() {
	err := env.Stop()
	gomega.Expect(err).To(gomega.BeNil())
})

var _ = ginkgo.Describe("Utils", func() {
	roleName := "default-tf-executor-clusterrole"
	ginkgo.It("CreateTerraformExecutorClusterRole", func() {
		err := CreateTerraformExecutorClusterRole(context.TODO(), k8sClient, roleName)
		gomega.Expect(err).To(gomega.BeNil())

		// Get and examine the role
		role := &rbacv1.ClusterRole{}
		err = k8sClient.Get(context.TODO(), client.ObjectKey{
			Name: roleName,
		}, role)
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(len(role.Rules)).To(gomega.Equal(2))
		gomega.Expect(role.Rules[0].Resources).To(gomega.Equal([]string{"secrets"}))
		gomega.Expect(role.Rules[0].Verbs).To(gomega.Equal([]string{"get", "list", "create", "update", "delete"}))
		gomega.Expect(role.Rules[1].Resources).To(gomega.Equal([]string{"leases"}))
		gomega.Expect(role.Rules[1].Verbs).To(gomega.Equal([]string{"get", "create", "update", "delete"}))
	})

	ginkgo.It("CreateTerraformExecutorClusterRoleBinding", func() {
		err := CreateTerraformExecutorClusterRoleBinding(context.TODO(), k8sClient, "default", roleName, "tf-executor-service-account")
		gomega.Expect(err).To(gomega.BeNil())
	})

})
