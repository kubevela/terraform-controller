package controllernamespace

import (
	"context"
	"strings"
	"time"

	types2 "github.com/oam-dev/terraform-controller/api/types"

	crossplane "github.com/oam-dev/terraform-controller/api/types/crossplane-runtime"
	//revive:disable-next-line:dot-imports
	. "github.com/onsi/ginkgo/v2"
	//revive:disable-next-line:dot-imports
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pkgClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/oam-dev/terraform-controller/api/v1beta2"
)

var _ = Describe("Restart with controller-namespace", func() {
	const (
		defaultNamespace    = "default"
		controllerNamespace = "terraform"
		chartNamespace      = "terraform"
	)
	var (
		controllerDeployMeta = types.NamespacedName{Name: "terraform-controller", Namespace: chartNamespace}
	)
	ctx := context.Background()

	// create k8s rest config
	restConf, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	k8sClient, err := pkgClient.New(restConf, pkgClient.Options{})
	s := k8sClient.Scheme()
	_ = v1beta2.AddToScheme(s)
	Expect(err).NotTo(HaveOccurred())

	configuration := &v1beta2.Configuration{
		ObjectMeta: v1.ObjectMeta{
			Name:      "e2e-for-ctrl-ns",
			Namespace: defaultNamespace,
		},
		Spec: v1beta2.ConfigurationSpec{
			HCL: `
resource "random_id" "server" {
  byte_length = 8
}
    
output "random_id" {
  value = random_id.server.hex
}`,
			InlineCredentials: true,
			WriteConnectionSecretToReference: &crossplane.SecretReference{
				Name:      "some-conn",
				Namespace: defaultNamespace,
			},
		},
	}
	AfterEach(func() {
		_ = k8sClient.Delete(ctx, configuration)
	})
	It("Restart with controller namespace", func() {
		By("apply configuration without --controller-namespace", func() {
			err = k8sClient.Create(ctx, configuration)
			Expect(err).NotTo(HaveOccurred())
			var cfg = &v1beta2.Configuration{}
			Eventually(func() error {
				err = k8sClient.Get(ctx, types.NamespacedName{Name: configuration.Name, Namespace: configuration.Namespace}, cfg)
				if err != nil {
					return err
				}
				if cfg.Status.Apply.State != types2.Available {
					return errors.Errorf("configuration is not available, status now: %s", cfg.Status.Apply.State)
				}
				return nil
			}, time.Second*60, time.Second*5).Should(Succeed())
		})
		By("restart controller with --controller-namespace", func() {
			ctrlDeploy := appv1.Deployment{}
			err = k8sClient.Get(ctx, controllerDeployMeta, &ctrlDeploy)
			Expect(err).NotTo(HaveOccurred())
			ctrlDeploy.Spec.Template.Spec.Containers[0].Args = append(ctrlDeploy.Spec.Template.Spec.Containers[0].Args, "--controller-namespace="+controllerNamespace)
			err := k8sClient.Update(ctx, &ctrlDeploy)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				err := k8sClient.Get(ctx, controllerDeployMeta, &ctrlDeploy)
				if err != nil {
					return err
				}
				if ctrlDeploy.Status.UnavailableReplicas == 1 {
					return errors.New("controller is not updated")
				}
				return nil
			}, time.Second*60, time.Second*5).Should(Succeed())

		})
		By("configuration should be still available", func() {
			// wait about half minute to check configuration's state isn't changed
			for i := 0; i < 30; i++ {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: configuration.Name, Namespace: configuration.Namespace,
				}, configuration)
				Expect(err).NotTo(HaveOccurred())
				time.Sleep(time.Second)
			}
		})
		By("restore controller", func() {
			ctrlDeploy := appv1.Deployment{}
			err = k8sClient.Get(ctx, controllerDeployMeta, &ctrlDeploy)
			Expect(err).NotTo(HaveOccurred())
			cmds := make([]string, 0)
			for _, cmd := range ctrlDeploy.Spec.Template.Spec.Containers[0].Args {
				if !strings.HasPrefix(cmd, "--controller-namespace") {
					cmds = append(cmds, cmd)
				}
			}
			ctrlDeploy.Spec.Template.Spec.Containers[0].Args = cmds
			err := k8sClient.Update(ctx, &ctrlDeploy)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
