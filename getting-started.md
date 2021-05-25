# Get started

## Prerequisites

- Helm Cli

Refer to [Helm official Doc](https://helm.sh/docs/intro/install/) to install `helm` Cli.

- Install Terraform Kubernetes Controller

Download the latest chart, like `terraform-controller-0.1.2.tgz`, from the latest [releases](https://github.com/oam-dev/terraform-controller/releases) and install it.

```shell
$ helm install terraform-controller terraform-controller-0.1.2.tgz
NAME: terraform-controller
LAST DEPLOYED: Mon Apr 26 15:55:35 2021
NAMESPACE: default
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

## For Alibaba Cloud

### Apply Provider configuration

```shell
$ export ALICLOUD_ACCESS_KEY=xxx; export ALICLOUD_SECRET_KEY=yyy

$ sh hack/prepare-alibaba-credentials.sh

$ kubectl get secret -n vela-system
NAME                                              TYPE                                  DATA   AGE
alibaba-account-creds                             Opaque                                1      11s

$ kubectl apply -f examples/alibaba/provider.yaml
provider.terraform.core.oam.dev/default created
```

### Apply Terraform Configuration

Apply Terraform configuration [configuration_hcl_oss.yaml](./examples/alibaba/configuration_hcl_oss.yaml) (JSON configuration [configuration_oss.yaml](./examples/alibaba/configuration_json_oss.yaml) is also supported) to provision an Alibaba OSS bucket.

```yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  name: alibaba-oss
spec:
  hcl: |
    resource "alicloud_oss_bucket" "bucket-acl" {
      bucket = var.bucket
      acl = var.acl
    }

    output "BUCKET_NAME" {
      value = "${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}"
    }

    variable "bucket" {
      description = "OSS bucket name"
      default = "vela-website"
      type = string
    }

    variable "acl" {
      description = "OSS bucket ACL, supported 'private', 'public-read', 'public-read-write'"
      default = "private"
      type = string
    }

  variable:
    bucket: "vela-website"
    acl: "private"

  writeConnectionSecretToRef:
    name: oss-conn
    namespace: default
```

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

### Looking into Configuration (optional)

- Watch the job to complete

```shell
$ kubectl get job
NAME               COMPLETIONS   DURATION   AGE
alibaba-oss-apply   1/1           12s        94s

$ kubectl get pod
NAME                     READY   STATUS      RESTARTS   AGE
alibaba-oss-apply-5c8b6   0/2     Completed   0          111s

$ kubectl logs alibaba-oss-rllx4 terraform-executor

Initializing the backend...

Initializing provider plugins...
- Finding latest version of hashicorp/alicloud...
- Installing hashicorp/alicloud v1.119.1...
- Installed hashicorp/alicloud v1.119.1 (signed by HashiCorp)

Terraform has created a lock file .terraform.lock.hcl to record the provider
selections it made above. Include this file in your version control repository
so that Terraform can guarantee to make the same selections by default when
you run "terraform init" in the future.


Warning: Additional provider information from registry

The remote registry returned warnings for
registry.terraform.io/hashicorp/alicloud:
- For users on Terraform 0.13 or greater, this provider has moved to
aliyun/alicloud. Please update your source in required_providers.

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
alicloud_oss_bucket.bucket-acl: Creating...
alicloud_oss_bucket.bucket-acl: Creation complete after 3s [id=vela-website]

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.

Outputs:

BUCKET_NAME = "vela-website.oss-cn-beijing.aliyuncs.com"
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

### Update Configuration

Change the OSS ACL to `public-read`.

```yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  name: alibaba-oss
spec:
  JSON: |
    ..

  variable:
    ...
    acl: "public-read"

```

### Delete Configuration

Delete the configuration will destroy the OSS cloud resource.

```shell
$ kubectl delete configuration.terraform.core.oam.dev alibaba-oss
configuration.terraform.core.oam.dev "alibaba-oss" deleted

$ ossutil ls oss://
Bucket Number is: 0

0.030917(s) elapsed
```

## For AWS

### Apply Provider configuration

```shell
$ export AWS_ACCESS_KEY_ID=xxx;export AWS_SECRET_ACCESS_KEY=yyy

$ sh hack/prepare-aws-credentials.sh

$ kubectl get secret -n vela-system
NAME                                              TYPE                                  DATA   AGE
aws-account-creds                                 Opaque                                1      52s

$ kubectl apply -f examples/aws/provider.yaml
provider.terraform.core.oam.dev/default created

$ kubectl apply -f examples/rbac.yaml
clusterrole.rbac.authorization.k8s.io/tf-clusterrole created
clusterrolebinding.rbac.authorization.k8s.io/tf-binding created
```

### Apply Terraform Configuration

Apply Terraform configuration [configuration_hcl_s3.yaml](./examples/aws/configuration_hcl_s3.yaml) to provision a s3 bucket.

```yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  name: aws-s3
spec:
  hcl: |
    resource "aws_s3_bucket" "bucket-acl" {
      bucket = var.bucket
      acl    = var.acl
    }

    output "BUCKET_NAME" {
      value = aws_s3_bucket.bucket-acl.bucket_domain_name
    }

    variable "bucket" {
      default = "vela-website"
    }

    variable "acl" {
      default = "private"
    }

  variable:
    bucket: "vela-website"
    acl: "private"

  writeConnectionSecretToRef:
    name: s3-conn
    namespace: default

```

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

# For GCP

### Apply Provider configuration

```shell
$ export GOOGLE_CREDENTIALS=xxx;export GOOGLE_PROJECT=yyy

$ sh hack/prepare-gcp-credentials.sh

$ kubectl get secret -n vela-system
NAME                                              TYPE                                  DATA   AGE
gcp-account-creds                                 Opaque                                1      52s

$ kubectl apply -f examples/gcp/provider.yaml
provider.terraform.core.oam.dev/default created

$ kubectl apply -f examples/rbac.yaml
clusterrole.rbac.authorization.k8s.io/tf-clusterrole created
clusterrolebinding.rbac.authorization.k8s.io/tf-binding created
```

### Apply Terraform Configuration

Apply Terraform configuration [configuration_hcl_s3.yaml](./examples/gcp/configuration_hcl_bucket.yaml) to provision a storage bucket.

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
NAME     AGE
aws-s3   6m48s

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
