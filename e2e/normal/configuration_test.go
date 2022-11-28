package normal

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"gotest.tools/assert"
	coreV1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/oam-dev/terraform-controller/controllers/client"
)

var (
	testConfigurationsInlineCredentials                        = "examples/random/configuration_random.yaml"
	testConfigurationsInlineCredentialsCustomBackendKubernetes = "examples/random/configuration_random_custom_backend_kubernetes.yaml"
	testConfigurationsRegression                               = []string{
		"examples/alibaba/eip/configuration_eip.yaml",
		"examples/alibaba/eip/configuration_eip_remote_in_another_namespace.yaml",
		"examples/alibaba/eip/configuration_eip_remote_subdirectory.yaml",
		"examples/alibaba/oss/configuration_hcl_bucket.yaml",
	}
	testConfigurationsForceDelete             = "examples/random/configuration_force_delete.yaml"
	testConfigurationsGitCredsSecretReference = "../examples/random/configuration_git_ssh.yaml"
	chartNamespace                            = "terraform"
)

type ConfigurationAttr struct {
	Name                   string
	YamlPath               string
	TFConfigMapName        string
	BackendStateSecretName string
	BackendStateSecretNS   string
	OutputsSecretName      string
	VariableSecretName     string
}

type TestContext struct {
	context.Context
	Configuration          *ConfigurationAttr
	BackendSecretNamespace string
	ClientSet              *kubernetes.Clientset
}

type DoFunc = func(ctx *TestContext)

type Injector struct {
	BeforeApplyConfiguration DoFunc
	CleanUp                  DoFunc
	// add more actions and check points if needed
}

func invoke(do DoFunc, ctx *TestContext) {
	if do != nil {
		do(ctx)
	}
}

func testBase(t *testing.T, configuration ConfigurationAttr, injector Injector, useCustomBackend bool) {
	klog.Infof("%s test begins……", configuration.Name)

	backendSecretNamespace := configuration.BackendStateSecretNS
	if backendSecretNamespace == "" {
		backendSecretNamespace = os.Getenv("TERRAFORM_BACKEND_NAMESPACE")
		if backendSecretNamespace == "" {
			backendSecretNamespace = "vela-system"
		}
	}

	clientSet, err := client.Init()
	assert.NilError(t, err)
	ctx := context.Background()

	testCtx := &TestContext{
		Context:                ctx,
		Configuration:          &configuration,
		BackendSecretNamespace: backendSecretNamespace,
		ClientSet:              clientSet,
	}

	defer invoke(injector.CleanUp, testCtx)

	klog.Info("1. Applying Configuration")
	invoke(injector.BeforeApplyConfiguration, testCtx)
	pwd, _ := os.Getwd()
	configuration.YamlPath = filepath.Join(pwd, "..", configuration.YamlPath)
	cmd := fmt.Sprintf("kubectl apply -f %s", configuration.YamlPath)
	err = exec.Command("bash", "-c", cmd).Start()
	assert.NilError(t, err)

	klog.Info("2. Checking Configuration status")
	for i := 0; i < 120; i++ {
		var fields []string
		output, err := exec.Command("bash", "-c", "kubectl get configuration").Output()
		assert.NilError(t, err)

		lines := strings.Split(string(output), "\n")
		for i, line := range lines {
			if i == 0 {
				continue
			}
			fields = strings.Fields(line)
			if len(fields) == 3 && fields[0] == configuration.Name && fields[1] == Available {
				goto continueCheck
			}
		}
		if i == 119 {
			t.Error("Configuration is not ready")
		}
		time.Sleep(time.Second * 5)
	}

continueCheck:
	klog.Info("3. Checking the status of Configs and Secrets")

	klog.Info("- Checking ConfigMap which stores .tf")
	_, err = clientSet.CoreV1().ConfigMaps("default").Get(ctx, configuration.TFConfigMapName, v1.GetOptions{})
	assert.NilError(t, err)

	if !useCustomBackend {
		klog.Info("- Checking Secret which stores Backend")
		_, err = clientSet.CoreV1().Secrets(backendSecretNamespace).Get(ctx, configuration.BackendStateSecretName, v1.GetOptions{})
		assert.NilError(t, err)
	}

	klog.Info("- Checking Secret which stores outputs")
	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, configuration.OutputsSecretName, v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("- Checking Secret which stores variables")
	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, configuration.VariableSecretName, v1.GetOptions{})
	assert.NilError(t, err)

	klog.Info("4. Deleting Configuration")
	cmd = fmt.Sprintf("kubectl delete -f %s", configuration.YamlPath)
	err = exec.Command("bash", "-c", cmd).Start()
	assert.NilError(t, err)

	klog.Info("5. Checking Configuration is deleted")
	for i := 0; i < 60; i++ {
		var (
			fields  []string
			existed bool
		)
		output, err := exec.Command("bash", "-c", "kubectl get configuration").Output()
		assert.NilError(t, err)

		lines := strings.Split(string(output), "\n")

		for j, line := range lines {
			if j == 0 {
				continue
			}
			fields = strings.Fields(line)
			if len(fields) == 3 && fields[0] == configuration.Name {
				existed = true
			}
		}
		if existed {
			if i == 59 {
				t.Error("Configuration is not deleted")
			}

			time.Sleep(time.Second * 5)
			continue
		} else {
			break
		}
	}

	klog.Info("6. Checking Secrets and ConfigMap which should all be deleted")

	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, configuration.OutputsSecretName, v1.GetOptions{})
	assert.Equal(t, kerrors.IsNotFound(err), true)

	_, err = clientSet.CoreV1().Secrets("default").Get(ctx, configuration.VariableSecretName, v1.GetOptions{})
	assert.Equal(t, kerrors.IsNotFound(err), true)

	if !useCustomBackend {
		_, err = clientSet.CoreV1().Secrets(backendSecretNamespace).Get(ctx, configuration.BackendStateSecretName, v1.GetOptions{})
		assert.Equal(t, kerrors.IsNotFound(err), true)
	}

	_, err = clientSet.CoreV1().ConfigMaps("default").Get(ctx, configuration.TFConfigMapName, v1.GetOptions{})
	assert.Equal(t, kerrors.IsNotFound(err), true)

	klog.Infof("%s test ends……", configuration.Name)
}

func TestInlineCredentialsConfiguration(t *testing.T) {
	configuration := ConfigurationAttr{
		Name:                   "random-e2e",
		YamlPath:               testConfigurationsInlineCredentials,
		TFConfigMapName:        "tf-random-e2e",
		BackendStateSecretName: "tfstate-default-random-e2e",
		OutputsSecretName:      "random-conn",
		VariableSecretName:     "variable-random-e2e",
	}
	testBase(t, configuration, Injector{}, false)
}

func TestInlineCredentialsConfigurationUseCustomBackendKubernetes(t *testing.T) {
	configuration := ConfigurationAttr{
		Name:                   "random-e2e-custom-backend-kubernetes",
		YamlPath:               testConfigurationsInlineCredentialsCustomBackendKubernetes,
		BackendStateSecretName: "tfstate-default-custom-backend-kubernetes",
		BackendStateSecretNS:   "a",
		TFConfigMapName:        "tf-random-e2e-custom-backend-kubernetes",
		OutputsSecretName:      "random-conn-custom-backend-kubernetes",
		VariableSecretName:     "variable-random-e2e-custom-backend-kubernetes",
	}
	beforeApply := func(ctx *TestContext) {
		cmd := exec.Command("bash", "-c", "kubectl create ns a")
		err := cmd.Run()
		assert.NilError(t, err)
	}
	cleanUp := func(ctx *TestContext) {
		cmd := exec.Command("bash", "-c", "kubectl delete ns a")
		err := cmd.Run()
		assert.NilError(t, err)
	}
	testBase(
		t,
		configuration,
		Injector{
			BeforeApplyConfiguration: beforeApply,
			CleanUp:                  cleanUp,
		},
		true,
	)
}

func TestForceDeleteConfiguration(t *testing.T) {
	klog.Info("1. Applying Configuration whose hcl is not valid")
	pwd, _ := os.Getwd()
	configuration := filepath.Join(pwd, "..", testConfigurationsForceDelete)
	cmd := fmt.Sprintf("kubectl apply -f %s", configuration)
	err := exec.Command("bash", "-c", cmd).Start()
	assert.NilError(t, err)

	klog.Info("2. Deleting Configuration")
	cmd = fmt.Sprintf("kubectl delete -f %s", configuration)
	err = exec.Command("bash", "-c", cmd).Start()
	assert.NilError(t, err)

	klog.Info("5. Checking Configuration is deleted")
	for i := 0; i < 60; i++ {
		var (
			fields  []string
			existed bool
		)
		output, err := exec.Command("bash", "-c", "kubectl get configuration").Output()
		assert.NilError(t, err)

		lines := strings.Split(string(output), "\n")

		for j, line := range lines {
			if j == 0 {
				continue
			}
			fields = strings.Fields(line)
			if len(fields) == 3 && fields[0] == "random-e2e-force-delete" {
				existed = true
			}
		}
		if existed {
			if i == 59 {
				t.Error("Configuration is not deleted")
			}

			time.Sleep(time.Second * 5)
			continue
		} else {
			break
		}
	}
}

func TestGitCredentialsSecretReference(t *testing.T) {
	configuration := ConfigurationAttr{
		Name:                   "random-e2e-git-creds-secret-ref",
		YamlPath:               testConfigurationsGitCredsSecretReference,
		TFConfigMapName:        "tf-random-e2e-git-creds-secret-ref",
		BackendStateSecretName: "tfstate-default-random-e2e-git-creds-secret-ref",
		OutputsSecretName:      "random-e2e-git-creds-secret-ref-conn",
		VariableSecretName:     "variable-random-e2e-git-creds-secret-ref",
	}

	clientSet, err := client.Init()
	assert.NilError(t, err)
	pwd, _ := os.Getwd()
	gitServer := filepath.Join(pwd, "..", "../examples/git-credentials")
	gitServerApplyCmd := fmt.Sprintf("kubectl apply -f %s", gitServer)
	gitServerDeleteCmd := fmt.Sprintf("kubectl apply -f %s", gitServer)

	beforeApply := func(ctx *TestContext) {
		err = exec.Command("bash", "-c", gitServerApplyCmd).Run()
		assert.NilError(t, err)

		klog.Info("- Checking git-server pod status")
		for i := 0; i < 120; i++ {
			pod, _ := clientSet.CoreV1().Pods("default").Get(ctx, "git-server", v1.GetOptions{})
			conditions := pod.Status.Conditions
			var index int
			for count, condition := range conditions {
				index = count
				if condition.Status == "True" && condition.Type == coreV1.PodReady {
					klog.Info("- pod=git-server ", condition.Type, "=", condition.Status)
					break
				}
			}
			if conditions[index].Status == "True" && conditions[index].Type == coreV1.PodReady {
				break
			}
			if i == 119 {
				t.Error("git-server pod is not running")
			}
			time.Sleep(10 * time.Second)
		}

		getKnownHostsCmd := "kubectl exec pod/git-server -- ssh-keyscan git-server"
		knownHosts, err := exec.Command("bash", "-c", getKnownHostsCmd).Output()
		assert.NilError(t, err)

		gitSshAuthSecretTmpl := filepath.Join(gitServer, "templates/git-ssh-auth-secret.tmpl")
		tmpl := template.Must(template.ParseFiles(gitSshAuthSecretTmpl))
		gitSshAuthSecretYaml := filepath.Join(gitServer, "git-ssh-auth-secret.yaml")
		gitSshAuthSecretYamlFile, err := os.Create(gitSshAuthSecretYaml)
		assert.NilError(t, err)
		err = tmpl.Execute(gitSshAuthSecretYamlFile, base64.StdEncoding.EncodeToString(knownHosts))
		assert.NilError(t, err)

		err = exec.Command("bash", "-c", gitServerDeleteCmd).Run()
		assert.NilError(t, err)
	}

	cleanUp := func(ctx *TestContext) {
		err = exec.Command("bash", "-c", gitServerDeleteCmd).Run()
		assert.NilError(t, err)
	}

	testBase(
		t,
		configuration,
		Injector{
			BeforeApplyConfiguration: beforeApply,
			CleanUp:                  cleanUp,
		},
		false,
	)
}

//func TestBasicConfigurationRegression(t *testing.T) {
//	var retryTimes = 120
//
//	klog.Info("0. Create namespace")
//	err := exec.Command("bash", "-c", "kubectl create ns abc").Start()
//	assert.NilError(t, err)
//
//	Regression(t, testConfigurationsRegression, retryTimes)
//}
