# Backup & Restore

This go module is a command line tool to back up and restore the configuration and the Terraform state.

It has two subcommands `backup` and `restore`.

## `restore`

The main usage of the `restore` subcommand is to import an "outside" Terraform instance (maybe created by the terraform command line or managed by another terraform-controller before) to the terraform-controller in the target kubernetes without recreating the cloud resources.

To use the `restore` subcommand, you should:

1. Prepare the `configuration.yaml` file and the `state.json` file whose content is the state json of the Terraform instance which would be imported.

2. Confirm that the environment variables set in terraform-controller are also set in the current execution environment.

3. Run the `restore` command: `go run main.go restore --configuration <path/to/your/configuration.yaml> --state <path/to/your/state.json>`

To find more usages of the `restore` subcommand, please run `go run main.go restore -h` for help.

### Example

You can find the example in the directory [examples/oss](examples/oss).

Assume that we have created an oss bucket named `restore-example` by applying the `main.tf` using the Terraform command line tool.

```hcl
provider "alicloud" {
  alias  = "bj-prod"
  region = "cn-beijing"
}

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
```

```shell
terraform init && terraform apply -f main.tf
```

We can get the Terraform state from the backend using `terraform state pull` and store it in the `state.json`. The Terraform state is a json string like the flowing:

```json
{
  "version": 4,
  "terraform_version": "1.1.9",
  "serial": 4,
  "lineage": "*******",
  "outputs": {
    "BUCKET_NAME": {
      "value": "restore-example.oss-cn-beijing.aliyuncs.com",
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
            "bucket": "restore-example",
            "cors_rule": [],
            "creation_date": "2022-05-25",
            "extranet_endpoint": "oss-cn-beijing.aliyuncs.com",
            "force_destroy": false,
            "id": "restore-example",
            "intranet_endpoint": "oss-cn-beijing-internal.aliyuncs.com",
            "lifecycle_rule": [],
            "location": "oss-cn-beijing",
            "logging": [],
            "logging_isenable": null,
            "owner": "*******",
            "policy": "",
            "redundancy_type": "LRS",
            "referer_config": [],
            "server_side_encryption_rule": [],
            "storage_class": "Standard",
            "tags": {},
            "transfer_acceleration": [],
            "versioning": [],
            "website": []
          },
          "sensitive_attributes": [],
          "private": "*******=="
        }
      ]
    }
  ]
}
```

Now, we need to restore(import) the Terraform instance into the target terraform-controller.

First, we need to prepare the `configuration.yaml`:

```yaml
apiVersion: terraform.core.oam.dev/v1beta2
kind: Configuration
metadata:
  name: alibaba-oss-bucket-hcl-restore-example
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
      default = "loheagn-terraform-controller-2"
      type = string
    }

    variable "acl" {
      description = "OSS bucket ACL, supported 'private', 'public-read', 'public-read-write'"
      default = "private"
      type = string
    }

  backend:
    backendType: kubernetes
    kubernetes:
      secret_suffix: 'ali-oss'
      namespace: 'default'

  variable:
    bucket: "restore-example"
    acl: "private"

  writeConnectionSecretToRef:
    name: oss-conn
    namespace: default

```

Second, we need to confirm that the environment variables set in terraform-controller are also set in the current execution environment. In this case, please make sure you have set `ALICLOUD_ACCESS_KEY` and `ALICLOUD_SECRET_KEY`.

Third, run the restore subcommand:

```shell
go run main.go restore --configuration examples/oss/configuration.yaml --state examples/oss/state.json
```

Finally, you can check the status of the configuration restored just now:

```shell
$ kubectl get configuration.terraform.core.oam.dev
NAME                                     STATE       AGE
alibaba-oss-bucket-hcl-restore-example   Available   13m
```

And you can check the logs of the `terraform-executor` in the pod of the "terraform apply" job:

```shell
$ kubectl logs alibaba-oss-bucket-hcl-restore-example-apply--1-b29d6 terraform-executor
alicloud_oss_bucket.bucket-acl: Refreshing state... [id=restore-example]

No changes. Your infrastructure matches the configuration.

Terraform has compared your real infrastructure against your configuration
and found no differences, so no changes are needed.

Apply complete! Resources: 0 added, 0 changed, 0 destroyed.

Outputs:

BUCKET_NAME = "restore-example.oss-cn-beijing.aliyuncs.com"
```

You can see the "No changes.". This shows that we did not recreate cloud resources during the restore process.
