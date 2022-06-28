# Backup & Restore

This go module is a command line tool to back up and restore the configuration and the Terraform state.

It has two subcommands `backup` and `restore`.

## `backup`

`backup` can be used to back up the Configuration objects managed by terraform-controller and their Terraform states.

The main usage of the `backup` subcommand is:

```shell
go main.go backup --configuration <name of the Configuration> --namespace <namespace of the Configuration>
```

Then you will get the `${configuration_name}_${configuration_namespace}_cofiguration.yaml` and the `${configuration_name}_${configuration_namespace}_state.json` in the workdir.

What's more, you can also use the `backup` subcommand to back up the Configurations created by the KubeVela Application:

```shell
go main.go backup --application <name of the Application>
```

The above command will scan all the components of the Application to find the Configurations and try to back up them. If you just want to back up a specific few components of the Application, you can use the `--component` argument:

```shell
go main.go backup --application <name of the Application> --component <component_1> <component_2>
```

Next, you can restore the Configuration and the Terraform state in another Kubernetes cluster using the `restore` subcommand.

## `restore`

The main usage of the `restore` subcommand is to import an "outside" Terraform instance (maybe created by the terraform command line or managed by another terraform-controller before) to the terraform-controller in the target Kubernetes without recreating the cloud resources.

To use the `restore` subcommand, you should:

1. Prepare the `configuration.yaml` file and the `state.json` file whose content is the state json of the Terraform instance which would be imported.

2. Confirm that the environment variables set in terraform-controller are also set in the current execution environment.

3. If you want to `resotre` the cloud resource managed by the Configuration explicitly, please run the `restore` command: `go run main.go restore --configuration <path/to/your/configuration.yaml> --state <path/to/your/state.json>`. On the other hand, ff you want to `resotre` the cloud resource managed by a KubeVela application, please run the `restore` command: `go run main.go restore --application <path/to/your/application.yaml> --component <the_cloud_resource_component_name> --state <path/to/your/state.json>`.

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

Then, you will see the output of the command like the flowing:

```text
2022/05/27 00:01:02 the Terraform backend was restored successfully
2022/05/27 00:01:02 try to restore the configuration......
2022/05/27 00:01:02 apply the configuration successfully, wait it to be available......
2022/05/27 00:01:02 the state of configuration is , wait it to be available......
2022/05/27 00:01:04 the state of configuration is ProvisioningAndChecking, wait it to be available......
2022/05/27 00:01:06 the state of configuration is ProvisioningAndChecking, wait it to be available......
2022/05/27 00:01:08 the state of configuration is ProvisioningAndChecking, wait it to be available......
2022/05/27 00:01:10 the state of configuration is ProvisioningAndChecking, wait it to be available......
2022/05/27 00:01:12 the state of configuration is ProvisioningAndChecking, wait it to be available......
2022/05/27 00:01:14 the state of configuration is ProvisioningAndChecking, wait it to be available......
2022/05/27 00:01:16 the configuration is available now
2022/05/27 00:01:16 try to print the log of the `terraform apply`......

alicloud_oss_bucket.bucket-acl: Refreshing state... [id=restore-example]

─────────────────────────────────────────────────────────────────────────────

No changes. Your infrastructure matches the configuration.

Your configuration already matches the changes detected above. If you'd like
to update the Terraform state to match, create and apply a refresh-only plan:
  terraform apply -refresh-only

Apply complete! Resources: 0 added, 0 changed, 0 destroyed.

Outputs:

BUCKET_NAME = "restore-example.oss-cn-beijing.aliyuncs.com"
```

The output is very clear, you can see the configuration is available.

And, you can see the `No changes.` in the log of the `terraform apply`. This shows that we did not recreate cloud resources during the restore process.
