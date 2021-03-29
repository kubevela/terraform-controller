/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"os"
	"os/exec"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	"github.com/zzxwill/terraform-controller/api/v1beta1"
	"github.com/zzxwill/terraform-controller/controllers/util"
)

const (
	// TerraformBaseLocation is the base directory to store all Terraform JSON files
	TerraformBaseLocation = ".vela/terraform/"
	// TerraformLog is the logfile name for terraform
	TerraformLog = "terraform.log"
	// Terraform image which can run `terraform init/plan/apply`
	TerraformImage = "hashicorp/terraform:full"
)

const DefaultNamespace = "vela-system"

// ConfigurationReconciler reconciles a Configuration object
type ConfigurationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=terraform.core.oam.dev,resources=configurations/status,verbs=get;update;patch

func (r *ConfigurationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	//_ = r.Log.WithValues("configuration", req.NamespacedName)
	ctx := context.Background()
	klog.InfoS("Reconciling Terraform Template...", "NamespacedName", req.NamespacedName)

	var configuration v1beta1.Configuration
	if err := r.Get(ctx, req.NamespacedName, &configuration); err != nil {
		if kerrors.IsNotFound(err) {
			err = nil
		}
		return ctrl.Result{}, err
	}

	tfVariable, err := getTerraformJSONVariable(configuration)
	if err != nil {
		return  ctrl.Result{}, errors.Wrap(err, fmt.Sprintf("failed to get Terraform JSON files from Configuration %s", configuration.Name))
	}

	job := batchv1.Job{
		TypeMeta: metav1.TypeMeta{Kind: "Job", APIVersion: "batch/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name: configuration.Name,
			Namespace: req.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: configuration.APIVersion,
				Kind:       configuration.Kind,
				Name:       configuration.Name,
				UID:        configuration.UID,
				Controller: pointer.BoolPtr(false),
			}},
		},
		Spec:batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Image: TerraformImage,
						Command: []string{""},
					}},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	tfJSONDir := filepath.Join(TerraformBaseLocation, configuration.Name)
	if _, err = os.Stat(tfJSONDir); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(tfJSONDir, 0750); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create directory for %s: %w", tfJSONDir, err)
		}
	}
	if err := ioutil.WriteFile(filepath.Join(tfJSONDir, "main.tf.json"), []byte(configuration.Spec.JSON), 0600); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to convert Terraform template: %w", err)
	}
	if err := ioutil.WriteFile(filepath.Join(tfJSONDir, "terraform.tfvars.json"), tfVariable, 0600); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to convert Terraform template: %w", err)
	}

	outputs, err := callTerraform(tfJSONDir)
	if err != nil {
		return ctrl.Result{}, err
	}
	cwd, err := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		return ctrl.Result{}, err
	}

	outputList := strings.Split(strings.ReplaceAll(string(outputs), " ", ""), "\n")
	if outputList[len(outputList)-1] == "" {
		outputList = outputList[:len(outputList)-1]
	}
	if err := generateSecretFromTerraformOutput(r.Client, outputList, req.Name, req.Namespace); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

type Variable map[string]interface{}

func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Configuration{}).
		Complete(r)
}

func getTerraformJSONVariable(c v1beta1.Configuration) ([]byte, error) {
	variables, err := util.RawExtension2Map(&c.Spec.Variable)
	if err != nil {
		return nil, err
	}
	tfVariableContent := map[string]Variable{}
	for k, v := range variables {
		tfVariableContent[k] = map[string]interface{}{"default": v}
	}

	tfVariable := map[string]interface{}{"variable": tfVariableContent}

	tfVariableJSON, err := json.Marshal(tfVariable)
	if err != nil {
		return nil, err
	}

	return tfVariableJSON,nil
}

func callTerraform(tfJSONDir string) ([]byte, error) {
	if err := os.Chdir(tfJSONDir); err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	cmd = exec.Command("bash", "-c", "terraform init")
	if err := RealtimePrintCommandOutput(cmd, TerraformLog); err != nil {
		return nil, err
	}

	cmd = exec.Command("bash", "-c", "terraform apply --auto-approve")
	if err := RealtimePrintCommandOutput(cmd, TerraformLog); err != nil {
		return nil, err
	}

	// Get output from Terraform
	cmd = exec.Command("bash", "-c", "terraform output")
	outputs, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return outputs, nil
}

// RealtimePrintCommandOutput prints command output in real time
// If logFile is "", it will prints the stdout, or it will write to local file
func RealtimePrintCommandOutput(cmd *exec.Cmd, logFile string) error {
	var writer io.Writer
	if logFile == "" {
		writer = io.MultiWriter(os.Stdout)
	} else {
		if _, err := os.Stat(filepath.Dir(logFile)); err != nil {
			return err
		}
		f, err := os.Create(filepath.Clean(logFile))
		if err != nil {
			return err
		}
		writer = io.MultiWriter(f)
	}
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// generateSecretFromTerraformOutput generates secret from Terraform output
func generateSecretFromTerraformOutput(k8sClient client.Client, outputList []string, name, namespace string) error {
	ctx := context.TODO()
	err := k8sClient.Create(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err == nil {
		return fmt.Errorf("namespace %s doesn't exist", namespace)
	}
	var cmData = make(map[string]string, len(outputList))
	for _, i := range outputList {
		line := strings.Split(i, "=")
		if len(line) != 2 {
			return fmt.Errorf("terraform output isn't in the right format")
		}
		k := strings.TrimSpace(line[0])
		v := strings.TrimSpace(line[1])
		if k != "" && v != "" {
			cmData[k] = v
		}
	}

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	var secret v1.Secret
	if err := k8sClient.Get(ctx, objectKey, &secret); err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("retrieving the secret from cloud resource %s hit an issue: %w", name, err)
	} else if err == nil {
		if err := k8sClient.Delete(ctx, &secret); err != nil {
			return fmt.Errorf("failed to store cloud resource %s output to secret: %w", name, err)
		}
	}

	secret = v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		StringData: cmData,
	}

	if err := k8sClient.Create(ctx, &secret); err != nil {
		return fmt.Errorf("failed to store cloud resource %s output to secret: %w", name, err)
	}
	return nil
}
