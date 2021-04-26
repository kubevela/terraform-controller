# Design

![](docs/resources/architecture.png)

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

It's taken upon by container oam-dev/docker-terraform:0.14.10, which is built from [oamdev/docker-terraform](https://github.com/oam-dev/docker-terraform.git).


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
