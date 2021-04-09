# Terraform Controller

Terraform Controller is a Kubernetes Controller for Terraform, which can address the requirement of [Using Terraform HCL as IaC module in KubeVela](https://github.com/oam-dev/kubevela/issues/698)

![](docs/resources/architecture.png)

# Design

## Components

### Provider

The `Provider` object is used to accept credentials from a Cloud provider, like Alibaba Cloud or AWS. For example, `ALICLOUD_ACCESS_KEY`,
`ALICLOUD_SECRET_KEY` from `Provider` will be used by `terraform init`.

This component is inspired by [Crossplane runtime](https://crossplane.io/), which can support various cloud providers.

### Configuration

The `Configuration` object is used to accept Terraform HCL/JSON configuration provisioning, updating and deletion. It covers
the whole lifecycle of a cloud resource.

- Configuration init component

This init component will retrieve HCL/JSON configuration from the object and store it to ConfigMap `aliyun-${ConfigurationName}-tf-input`.

During creation stage, it will mount the ConfigMap to a volume and copy the Terraform configuration file to the working directory.

During update stage, it will mount Terraform state file ConfigMap `aliyun-${ConfigurationName}-tf-state`, which will be generated
after a cloud resource is successfully provisioned, to the volume and copy it to the working directory.

This component is taken upon by container `pause`.

- Terraform configuration executor component

This executor component will perform `terrform init` and `terraform apply`. After a cloud resource is successfully provisioned,
Terraform state file will be generated.

This executor is job, which has the ability to retry and auto-recovery from failures.

It's taken upon by container zzxwill/docker-terraform:0.14.10, which is built from [zzxwill/broadinstitute-docker-terraform](https://github.com/zzxwill/broadinstitute-docker-terraform.git).


- Terraform state file retriever

This component is relatively simple, which will monitor the generation of Terraform state file. Upon the state file is
generated, it will store the file content to ConfigMap `aliyun-${ConfigurationName}-tf-state`, which will be used during
`Configuration` update and deletion stage.

This component is taken upon by the container zzxwill/terraform-tfstate-retriever:v0.2, which  built from [terraform-tfstate-retriever](https://github.com/zzxwill/terraform-tfstate-retriever).

## Technical alternatives

### Why taking Crossplane ProviderConfiguration as cloud credentials Provider?

As Terraform controller is intended to support various Cloud providers, like AWS, Azure, Alibaba Cloud, GCP, and VMWare.
Crossplane new `ProviderConfiguration` is known as it mature model for these cloud providers. By utilizing the model, this
controller can support various cloud providers at the very first day.

### Why choosing ConfigMap as the storage system over cloud shared disks or Object storage system?

By using ConfigMap to store terraform configuration files and generated state file will be a generic way for nearly all 
Kubernetes clusters. 

By using cloud shared volumes/Object Storage System(like Alibaba OSS, and AWS S3), it's straight forward as terraform
HCL/JSON configuration and generated state are files. But we have to adapt to various cloud providers with various storage
solution like cloud disk or OSS to Alibaba Cloud, s3 to AWS.

Here is a drawback for the choice: we have to grant the Pod in the Job to create ConfigMaps.

# Get started

Let's manage cloud resource on Alibaba Cloud as an example.

## Locally run Terraform Controller

Get the codebase from [release v0.1-alpha.1](https://github.com/zzxwill/terraform-controller/releases/tag/v0.1-alpha.1),
and run it locally.

## Apply Provider configuration

```shell
$ export ALICLOUD_ACCESS_KEY=xxx; export ALICLOUD_SECRET_KEY=yyy

$ sh hack/prepare-alibaba-credentials.sh

$ kubectl get secret -n vela-system
NAME                                              TYPE                                  DATA   AGE
alibaba-account-creds                             Opaque                                1      11s

$ k apply -f examples/provider.yaml
provider.terraform.core.oam.dev/default created
```

## Authenticate pods to create ConfigMaps

Terraform state file is essential to update or destroy cloud resources. After terraform execution completes, its state file
needs to be stored to a ConfigMap.

```shell
✗ kubectl apply -f examples/rbac.yaml
clusterrole.rbac.authorization.k8s.io/tf-clusterrole created
clusterrolebinding.rbac.authorization.k8s.io/tf-binding created
```

## Apply Terraform Configuration

Apply Terraform configuration [configuration_oss.yaml](./examples/configuration_oss.yaml) to provision an Alibaba OSS bucket.

```yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  name: aliyun-oss
spec:
  JSON: |
    {
      "resource": {
        "alicloud_oss_bucket": {
          "bucket-acl": {
            "bucket": "${var.bucket}",
            "acl": "${var.acl}"
          }
        }
      },
      "output": {
        "BUCKET_NAME": {
          "value": "${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}"
        }
      },
      "variable": {
        "bucket": {
          "default": "poc"
        },
        "acl": {
          "default": "private"
        }
      }
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
aliyun-oss   1h

$ kubectl get configuration.terraform.core.oam.dev aliyun-oss -o yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"terraform.core.oam.dev/v1beta1","kind":"Configuration","metadata":{"annotations":{},"name":"aliyun-oss","namespace":"default"},"spec":{"JSON":"{\n  \"resource\": {\n    \"alicloud_oss_bucket\": {\n      \"bucket-acl\": {\n        \"bucket\": \"${var.bucket}\",\n        \"acl\": \"${var.acl}\"\n      }\n    }\n  },\n  \"output\": {\n    \"BUCKET_NAME\": {\n      \"value\": \"${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}\"\n    }\n  },\n  \"variable\": {\n    \"bucket\": {\n      \"default\": \"poc\"\n    },\n    \"acl\": {\n      \"default\": \"private\"\n    }\n  }\n}\n","variable":{"acl":"private","bucket":"vela-website"},"writeConnectionSecretToRef":{"name":"oss-conn","namespace":"default"}}}
  creationTimestamp: "2021-04-02T08:17:08Z"
  generation: 2
spec:
  JSON: |
    {
      "resource": {
        "alicloud_oss_bucket": {
          "bucket-acl": {
            "bucket": "${var.bucket}",
            "acl": "${var.acl}"
          }
        }
      },
      "output": {
        "BUCKET_NAME": {
          "value": "${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}"
        }
      },
      "variable": {
        "bucket": {
          "default": "poc"
        },
        "acl": {
          "default": "private"
        }
      }
    }
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

## Looking into Configuration (optional)

### Watch the job to complete

```shell
$ kubectl get job
NAME               COMPLETIONS   DURATION   AGE
aliyun-oss-apply   1/1           12s        94s

$ kubectl get pod
NAME                     READY   STATUS      RESTARTS   AGE
aliyun-oss-apply-5c8b6   0/2     Completed   0          111s

$ kubectl logs aliyun-oss-rllx4 terraform-executor

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

### Check whether Terraform state file is stored

```shell
✗ kubectl get cm | grep aliyun-oss
aliyun-oss-tf-input      1      16m
aliyun-oss-tf-state      1      11m

✗ kubectl get cm aliyun-oss-tf-state -o yaml
apiVersion: v1
data:
  terraform.tfstate: |
    {
      "version": 4,
      "terraform_version": "0.14.9",
      "serial": 2,
      "lineage": "61cbded2-6323-0f83-823d-9c40c000b91d",
      "outputs": {
        "BUCKET_NAME": {
          "value": "vela-website.oss-cn-beijing.aliyuncs.com",
          "type": "string"
        }
      },
      "resources": [
        {
          "mode": "managed",
          "type": "alicloud_oss_bucket",
          "name": "bucket-acl",
          "provider": "provider[\"registry.terraform.io/hashicorp/alicloud\"]",
          "instances": [
            {
              "schema_version": 0,
              "attributes": {
                "acl": "private",
                "bucket": "vela-website",
                "cors_rule": [],
                "creation_date": "2021-04-02",
                "extranet_endpoint": "oss-cn-beijing.aliyuncs.com",
                "force_destroy": false,
                "id": "vela-website",
                "intranet_endpoint": "oss-cn-beijing-internal.aliyuncs.com",
                "lifecycle_rule": [],
                "location": "oss-cn-beijing",
                "logging": [],
                "logging_isenable": null,
                "owner": "1874279259696164",
                "policy": "",
                "redundancy_type": "LRS",
                "referer_config": [],
                "server_side_encryption_rule": [],
                "storage_class": "Standard",
                "tags": null,
                "versioning": [],
                "website": []
              },
              "sensitive_attributes": [],
              "private": "bnVsbA=="
            }
          ]
        }
      ]
    }
kind: ConfigMap
metadata:
  creationTimestamp: "2021-04-02T03:37:31Z"
  managedFields:
  - apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1:
      f:data:
        .: {}
        f:terraform.tfstate: {}
    manager: terraform-tfstate-retriever
    operation: Update
    time: "2021-04-02T03:37:31Z"
  name: aliyun-oss-tf-state
  namespace: default
  resourceVersion: "33145818"
  selfLink: /api/v1/namespaces/default/configmaps/aliyun-oss-tf-state
  uid: 762b1912-1f8f-428c-a4c7-2a7297375579
```

### Check the generated connection secret

```shell
$ kubectl get secret oss-conn
NAME       TYPE     DATA   AGE
oss-conn   Opaque   1      2m41s
```

## Update Configuration

Change the OSS ACL to `public-read`.

```yaml
apiVersion: terraform.core.oam.dev/v1beta1
kind: Configuration
metadata:
  name: aliyun-oss
spec:
  JSON: |
    ..

  variable:
    ...
    acl: "public-read"

```

## Delete Configuration

Delete the configuration will destroy the OSS cloud resource.

```shell
$ kubectl delete configuration.terraform.core.oam.dev aliyun-oss
configuration.terraform.core.oam.dev "aliyun-oss" deleted

$ ossutil ls oss://
Bucket Number is: 0

0.030917(s) elapsed
```