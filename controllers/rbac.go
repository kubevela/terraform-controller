package controllers

import (
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createTerraformExecutorClusterRole(ctx context.Context, k8sClient client.Client, clusterRoleName string) error {
	var clusterRole = rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "create", "update", "delete"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "create", "update", "delete"},
			},
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: clusterRoleName}, &rbacv1.ClusterRole{}); err != nil {
		if kerrors.IsNotFound(err) {
			if err := k8sClient.Create(ctx, &clusterRole); err != nil {
				return errors.Wrap(err, "failed to create ClusterRole for Terraform executor")
			}
		}
	}
	return nil
}

func createTerraformExecutorClusterRoleBinding(ctx context.Context, k8sClient client.Client, namespace, clusterRoleName, serviceAccountName string) error {
	var crbName = "tf-executor-role-binding"
	var clusterRoleBinding = rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      crbName,
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: crbName}, &rbacv1.ClusterRoleBinding{}); err != nil {
		if kerrors.IsNotFound(err) {
			if err := k8sClient.Create(ctx, &clusterRoleBinding); err != nil {
				return errors.Wrap(err, "failed to create ClusterRoleBinding for Terraform executor")
			}
		}
	}
	return nil
}

func createTerraformExecutorServiceAccount(ctx context.Context, k8sClient client.Client, namespace, serviceAccountName string) error {
	var serviceAccount = v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: serviceAccountName, Namespace: namespace}, &v1.ServiceAccount{}); err != nil {
		if kerrors.IsNotFound(err) {
			if err := k8sClient.Create(ctx, &serviceAccount); err != nil {
				return errors.Wrap(err, "failed to create ServiceAccount for Terraform executor")
			}
		}
	}
	return nil
}
