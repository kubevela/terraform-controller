package util

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func init() {
	patches := ApplyMethod(reflect.TypeOf(&envtest.Environment{}), "Start", func(_ *envtest.Environment) (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	patches.ApplyMethod(reflect.TypeOf(&envtest.Environment{}), "Stop", func(_ *envtest.Environment) error {
		return nil
	})
	patches.ApplyFunc(CreateTerraformExecutorClusterRole, func(ctx context.Context, c client.Client, name string) error {
		role := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Rules: []rbacv1.PolicyRule{
				{Resources: []string{"secrets"}, Verbs: []string{"get", "list", "create", "update", "delete"}},
				{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "create", "update", "delete"}},
			},
		}
		return c.Create(ctx, role)
	})
	patches.ApplyFunc(CreateTerraformExecutorClusterRoleBinding, func(ctx context.Context, c client.Client, namespace, clusterRoleName, serviceAccountName string) error {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-tf-executor-clusterrole-binding", namespace), Namespace: namespace},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: clusterRoleName},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: serviceAccountName, Namespace: namespace}},
		}
		return c.Create(ctx, crb)
	})
	patches.ApplyFunc(client.New, func(_ *rest.Config, _ client.Options) (client.Client, error) {
		return fake.NewClientBuilder().Build(), nil
	})
}

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}
