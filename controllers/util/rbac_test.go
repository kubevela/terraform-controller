package util

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	env       envtest.Environment
	k8sClient client.Client
)

var _ = BeforeSuite(func() {
	env = envtest.Environment{}
	cfg, err := env.Start()
	Expect(err).To(BeNil())
	Expect(cfg).ToNot(BeNil())
	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	err := env.Stop()
	Expect(err).To(BeNil())
})

var _ = Describe("Utils", func() {
	roleName := "default-tf-executor-clusterrole"
	It("CreateTerraformExecutorClusterRole", func() {
		err := CreateTerraformExecutorClusterRole(context.TODO(), k8sClient, roleName)
		Expect(err).To(BeNil())

		// Get and examine the role
		role := &rbacv1.ClusterRole{}
		err = k8sClient.Get(context.TODO(), client.ObjectKey{
			Name: roleName,
		}, role)
		Expect(err).To(BeNil())
		Expect(len(role.Rules)).To(Equal(2))
		Expect(role.Rules[0].Resources).To(Equal([]string{"secrets"}))
		Expect(role.Rules[0].Verbs).To(Equal([]string{"get", "list", "create", "update", "delete"}))
		Expect(role.Rules[1].Resources).To(Equal([]string{"leases"}))
		Expect(role.Rules[1].Verbs).To(Equal([]string{"get", "create", "update", "delete"}))
	})

	It("CreateTerraformExecutorClusterRoleBinding", func() {
		err := CreateTerraformExecutorClusterRoleBinding(context.TODO(), k8sClient, "default", roleName, "tf-executor-service-account")
		Expect(err).To(BeNil())
	})

})
