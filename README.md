# terraform-controller
Kubernetes Terraform Controller is inspired by [Crossplane runtime](https://crossplane.io/).

## Terraform related docker

- zzxwill/docker-terraform:0.14.9

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

✗ kubectl get job
NAME         COMPLETIONS   DURATION   AGE
aliyun-oss   1/1           26s        4m58s

✗ kubectl get pod
NAME               READY   STATUS      RESTARTS   AGE
aliyun-oss-ksx5m   0/1     Error       0          5m25s
aliyun-oss-szklr   0/1     Completed   0          5m11s
aliyun-oss-z8hxc   0/1     Error       0          5m21s

✗ kubectl logs -f aliyun-oss-szklr

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
