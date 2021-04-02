# terraform-controller
Kubernetes Terraform Controller is inspired by [Crossplane runtime](https://crossplane.io/).

## Terraform related docker

- Terraform binary Docker 
  
  zzxwill/docker-terraform:0.14.9 built from [docker-terraform](https://github.com/zzxwill/docker-terraform/tree/long-run-container)


- Terraform terraform.tfstate retriever

  zzxwill/terraform-tfstate-retriever:v0.1 built from [terraform-tfstate-retriever](https://github.com/zzxwill/terraform-tfstate-retriever) 
  

## Terraform Provider configuration

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

Terraform state file is essential, after terraform execution completes, its state file needs to be stored to ConfigMaps, 
shared cloud disks, or object strages.

In this solution, ConfigMaps is chosen.

```shell
✗ kubectl apply -f examples/rbac.yaml
clusterrole.rbac.authorization.k8s.io/tf-clusterrole created
clusterrolebinding.rbac.authorization.k8s.io/tf-binding created
```

## Terraform Configuration

Apply Terraform configuration [OSS](./examples/configuration_oss.yaml) to provision an Alibaba OSS bucket.

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
            "acl": "private"
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
        }
      }
    }

  variable:
    bucket: "vela-website"
```

```shell
✗ kubectl get configuration.terraform.core.oam.dev
NAME         AGE
aliyun-oss   6d1h
```

## Watch the job to complete

```shell
✗ kubectl get job
NAME         COMPLETIONS   DURATION   AGE
aliyun-oss   1/1           5m25s      8m7s

✗ kubectl get pod
NAME               READY   STATUS      RESTARTS   AGE
aliyun-oss-rllx4   0/2     Completed   0          3m

✗ kubectl logs aliyun-oss-rllx4 terraform-executor

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

## Check whether Terraform state file is stored

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
