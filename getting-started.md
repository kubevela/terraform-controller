# Get started

You can choose either way to provision cloud resources with Terraform Controller.

## Standalone Terraform Controller

### Prerequisites

- Helm Cli

Refer to [Helm official Doc](https://helm.sh/docs/intro/install/) to install `helm v3` Cli.

- Install Kubernetes Terraform Controller Chart

```shell
$ helm repo add kubevela-addons https://charts.kubevela.net/addons
"kubevela-addons" has been added to your repositories

$ helm upgrade --install terraform-controller -n terraform --create-namespace kubevela-addons/terraform-controller
Release "terraform-controller" does not exist. Installing it now.
NAME: terraform-controller
LAST DEPLOYED: Mon Aug 30 11:23:47 2021
NAMESPACE: terraform
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

### Authenticate Cloud Provider and Create a Cloud Resource

#### Alibaba Cloud

<details>
<summary>Sample</summary>

- Apply Provider credentials


```shell
$ export ALICLOUD_ACCESS_KEY=xxx; export ALICLOUD_SECRET_KEY=yyy
```

If you'd like to use Alicloud Security Token Service, also export `ALICLOUD_SECURITY_TOKEN`.
```shell
$ export ALICLOUD_SECURITY_TOKEN=zzz
```

```shell
$ make alibaba
```

- Apply Terraform Configuration

Apply Terraform Configuration [configuration_hcl_oss.yaml](./examples/alibaba/oss/configuration_hcl_bucket.yaml) (JSON configuration [configuration_oss.yaml](./examples/alibaba/oss/configuration_json_bucket.yaml) is also supported) to provision an Alibaba OSS bucket.

```shell
$ kubectl get configuration.terraform.core.oam.dev
NAME         AGE
alibaba-oss   1h

$ kubectl get configuration.terraform.core.oam.dev alibaba-oss -o yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"terraform.core.oam.dev/v1beta1","kind":"Configuration","metadata":{"annotations":{},"name":"alibaba-oss","namespace":"default"},"spec":{"JSON":"{\n  \"resource\": {\n    \"alicloud_oss_bucket\": {\n      \"bucket-acl\": {\n        \"bucket\": \"${var.bucket}\",\n        \"acl\": \"${var.acl}\"\n      }\n    }\n  },\n  \"output\": {\n    \"BUCKET_NAME\": {\n      \"value\": \"${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}\"\n    }\n  },\n  \"variable\": {\n    \"bucket\": {\n      \"default\": \"poc\"\n    },\n    \"acl\": {\n      \"default\": \"private\"\n    }\n  }\n}\n","variable":{"acl":"private","bucket":"vela-website"},"writeConnectionSecretToRef":{"name":"oss-conn","namespace":"default"}}}
  creationTimestamp: "2021-04-02T08:17:08Z"
  generation: 2
spec:
  ...
  variable:
    acl: private
    bucket: vela-website
  writeConnectionSecretToRef:
    name: oss-conn
    namespace: default
status:
  outputs:
    BUCKET_NAME:
      type: string
      value: vela-website.oss-cn-beijing.aliyuncs.com
  state: provisioned
```

OSS bucket is provisioned.

```shell
$ ossutil ls oss://
CreationTime                                 Region    StorageClass    BucketName
2021-04-10 00:42:09 +0800 CST        oss-cn-beijing        Standard    oss://vela-website
Bucket Number is: 1

0.146789(s) elapsed
```

- Check the generated connection secret

```shell
$ kubectl get secret oss-conn
NAME       TYPE     DATA   AGE
oss-conn   Opaque   1      2m41s
```

</details>


#### AWS

<details>
<summary>Sample</summary>

- Apply Provider credentials

```shell
$ export AWS_ACCESS_KEY_ID=xxx;export AWS_SECRET_ACCESS_KEY=yyy
```

If you'd like to use AWS session token for temporary credentials, please export `AWS_SESSION_TOKEN`.
```shell
$ export AWS_SESSION_TOKEN=zzz
```

```
$ make aws
```

- Apply Terraform Configuration

Apply Terraform configuration [configuration_hcl_s3.yaml](./examples/aws/configuration_hcl_s3.yaml) to provision a s3 bucket.

```shell
$ kubectl get configuration.terraform.core.oam.dev
NAME     AGE
aws-s3   6m48s

$ kubectl describe configuration.terraform.core.oam.dev aws-s3
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
...
  Write Connection Secret To Ref:
    Name:       s3-conn
    Namespace:  default
Status:
  Outputs:
    BUCKET_NAME:
      Type:   string
      Value:  vela-website.s3.amazonaws.com
  State:      provisioned

$ kubectl get secret s3-conn
NAME      TYPE     DATA   AGE
s3-conn   Opaque   1      7m37s

$ aws s3 ls
2021-04-12 19:03:32 vela-website
```

</details>


#### Azure

<details>
<summary>Sample</summary>

- Apply Provider credentials

```shell
$ export ARM_SUBSCRIPTION_ID="aaa"
export ARM_TENANT_ID="bbb"
export ARM_CLIENT_ID="ccc"
export ARM_CLIENT_SECRET="ddd"
```

```
$ make aws
```

- Apply Terraform Configuration

Apply Terraform configuration [configuration_database_mariadb.yaml](./examples/azure/configuration_database_mariadb.yaml) to provision a Mariadb instance.

</details>

#### GCP

<details>
<summary>Sample</summary>

- Apply Provider credentials

For authentication with GCP, the GOOGLE_CREDENTIALS variable containing the Google authentication JSON must be exported.
At this time, the file path is not supported.

```shell
$ export GOOGLE_CREDENTIALS='{ "type": "service_account", "project_id": "example-project-123456", "private_key_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "private_key": "-----BEGIN PRIVATE KEY-----\\n-----END PRIVATE KEY-----\n", "client_email": "test@developer.gserviceaccount.com", "client_id": "123456789012345678901", "auth_uri": "https://accounts.google.com/o/oauth2/auth", "token_uri": "https://oauth2.googleapis.com/token", "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs", "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%40developer.gserviceaccount.com" }'
$ export GOOGLE_PROJECT=yyy

$ sh hack/prepare-gcp-credentials.sh

$ kubectl get secret -n vela-system
NAME                                              TYPE                                  DATA   AGE
gcp-account-creds                                 Opaque                                1      52s

$ kubectl apply -f examples/gcp/provider.yaml
provider.terraform.core.oam.dev/default created
```

- Apply Terraform Configuration

Apply Terraform configuration [configuration_hcl_bucket.yaml](./examples/gcp/configuration_hcl_bucket.yaml) to provision a storage bucket.

```yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  name: gcp-bucket
spec:
  hcl: |
    resource "google_storage_bucket" "bucket" {
      name = var.bucket
    }

    output "BUCKET_URL" {
      value = google_storage_bucket.bucket.url
    }

    variable "bucket" {
      default = "vela-website"
    }

  variable:
    bucket: "vela-website"
    acl: "private"

  writeConnectionSecretToRef:
    name: bucket-conn
    namespace: default
```

```shell
$ kubectl get configuration.terraform.core.oam.dev
NAME         AGE
gcp-bucket   6m48s

$ kubectl describe configuration.terraform.core.oam.dev gcp-bucket
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
...
  Write Connection Secret To Ref:
    Name:       bucket-conn
    Namespace:  default
Status:
  Outputs:
    BUCKET_URL:
      Type:   string
      Value:  gs://vela-website
  State:      provisioned

$ kubectl get secret bucket-conn
NAME      TYPE     DATA   AGE
bucket-conn   Opaque   1      7m37s
```

</details>


#### VMware vSphere

<details>
<summary>Sample</summary>

- Apply Provider credentials

```shell script
$ export VSPHERE_USER=xxx
$ export VSPHERE_PASSWORD=yyy
$ export VSPHERE_SERVER=zzz
# If you have a self-signed cert, you will need this.
$ export VSPHERE_ALLOW_UNVERIFIED_SSL=true

$ sh hack/prepare-vsphere-credentials.sh

$ kubectl get secret -n vela-system
NAME                             TYPE                                  DATA   AGE
vsphere-account-creds            Opaque                                1      1m

$ kubectl apply -f examples/vsphere/provider.yaml
```

- Apply Terraform configuration

Apply Terraform configuration [configuration_hcl_folder.yaml](./examples/vsphere/provider.yaml) to provision a folder.

```yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  name: vsphere-folder
spec:
  hcl: |
    #############
    # Variables #
    #############
    variable "vsphere-datacenter" {
      type        = string
      description = "VMware vSphere datacenter"
    }

    variable "folder-name" {
      type        = string
      description = "The name of folder"
    }

    variable "folder-type" {
      type        = string
      description = "The type of folder"
    }

    ##########
    # Folder #
    ##########

    data "vsphere_datacenter" "dc" {
      name = var.vsphere-datacenter
    }

    resource "vsphere_folder" "folder" {
      path          = var.folder-name
      type          = var.folder-type
      datacenter_id = data.vsphere_datacenter.dc.id
    }

    output "folder" {
        value       = "folder-${var.folder-type}-${var.folder-name}"
    }

  variable:
    vsphere-datacenter: Datacenter01
    folder-name: test
    folder-type: vm

  writeConnectionSecretToRef:
    name: folder-outputs
    namespace: default

  providerRef:
    name: vsphere
```

```shell script
$ kubectl get configuration.terraform.core.oam.dev
NAME             STATE       AGE
vsphere-folder   Available   17m

$ kubectl describe configuration.terraform.core.oam.dev vsphere-folder
Name:         vsphere-folder
Namespace:    default
Labels:       <none>
Annotations:  API Version:  terraform.core.oam.dev/v1beta1
Kind:         Configuration
...
Status:
  Message:  Cloud resources are deployed and ready to use.
  Outputs:
    Folder:
      Type:   string
      Value:  folder-vm-test
  State:      Available
Events:       <none>

$ kubectl get secret folder-outputs
NAME         TYPE     DATA   AGE
vm-outputs   Opaque   1      18m
```

</details>

#### Elastic Cloud

<details>
<summary>Sample</summary>

- Apply Provider credentials

To interact with the EC Terraform provider an API key is expected. Please see Terraform EC provider documentation for [generating API keys](https://registry.terraform.io/providers/elastic/ec/latest/docs).
```shell
$ export EC_API_KEY=xxx

$ sh hack/prepare-ec-credentials.sh

$ kubectl get secret -n vela-system
NAME                                              TYPE                                  DATA   AGE
ec-account-creds                                 Opaque                                1      52s

$ kubectl apply -f examples/ec/provider.yaml
provider.terraform.core.oam.dev/ec created
```

- Apply Terraform Configuration

Apply Terraform configuration [configuration_hcl_ecproject.yaml](./examples/ec/configuration_hcl_ecdeployment.yaml) to provision a Elastic Cloud project.
```yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  name: ec-deployment
spec:
  hcl: |
    terraform {
      required_providers {
        ec = {
          source  = "elastic/ec"
          version = "0.2.1"
        }
      }
    }

    data "ec_stack" "latest" {
      version_regex = "latest"
      region        = var.ec_region
    }

    resource "ec_deployment" "project" {
      name = var.project_name

      region                 = var.ec_region
      version                = data.ec_stack.latest.version
      deployment_template_id = "gcp-io-optimized"

      elasticsearch {
        autoscale = "true"
      }

      kibana {}
    }

    output "ES_HTTPS_ENDPOINT" {
      value = ec_deployment.project.elasticsearch[0].https_endpoint
    }

    output "ES_PASSWORD" {
      value = ec_deployment.project.elasticsearch_password
      sensitive = true
    }

    variable "ec_region" {
      default = "gcp-us-west1"
    }

    variable "project_name" {
      default = "example"
    }
  variable:
    project_name: "es-project-1"

  writeConnectionSecretToRef:
    name: es-connection
    namespace: default

  providerRef:
    name: default
```

```shell
$ kubectl get configuration.terraform.core.oam.dev
NAME           AGE
ec-deployment  6m48s

$ kubectl describe configuration.terraform.core.oam.dev ec-deployment
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
...
  Write Connection Secret To Ref:
    Name:       es-connection
    Namespace:  default
Status:
  Outputs:
    ES_HTTPS_ENDPOINT:
      Type:   string
      Value:  https://example1234.us-west1.gcp.cloud.es.io:9243
    ES_PASSWORD:
      Type:   string
      Value: <sensitive>
  State:      provisioned

$ kubectl get secret es-connection
NAME            TYPE     DATA   AGE
es-connection   Opaque   1      7m37s
```

</details>


## KubeVela Terraform Addon

Terraform Controller is well integrated in [KubeVela](https://github.com/oam-dev/kubevela) as a Terraform addon. Enabling
Terraform addon and Terraform provider addon is the simplest way.

- Install Terraform Controller

```shell
$ vela addon enable terraform
```

For more detailed, please refer to [install KubeVela addon](https://kubevela.io/docs/reference/addons/terraform).

- Authenticate Terraform Controller with credentials of Cloud provider

Please refer to [enable KubeVela Terraform Provider addon](https://kubevela.io/docs/reference/addons/terraform) to authenticate
Terraform Controller with the credentials of a Cloud provider.

For example, you can authenticate Alibaba Cloud with the following command.

```shell
$ vela addon enable terraform-alibaba ALICLOUD_ACCESS_KEY=<xxx> ALICLOUD_SECRET_KEY=<yyy> ALICLOUD_REGION=<region>
```

- Provision and Consume cloud resources

Try to provision and consume cloud resources by KubeVela [Cli](https://kubevela.io/docs/end-user/components/cloud-services/provider-and-consume-cloud-services) or [VelaUX](https://kubevela.io/docs/next/tutorials/consume-cloud-services).
