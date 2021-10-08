# Get started

# Install Terraform Controller

Installing Terraform Controller as a [KuberVela](https://github.com/oam-dev/kubevela) addon by [vela cli](https://kubevela.io/docs/install#4-optional-enable-addons) is the simplest way.

```shell
vela addon enable terraform
```

For more detailed, please refer to [install KubeVela addon](https://kubevela.io/docs/install#4-optional-enable-addons).


# Authenticate Terraform Controller with credentials of Cloud provider

Please refer to [enable KubeVela Terraform Provider addon](https://kubevela.io/docs/install#4-optional-enable-addons) to authenticate
Terraform Controller with the credentials of a Cloud provider.

For example, you can authenticate Alibaba Cloud with the following command.

```shell
vela addon enable terraform/provider-alibaba ALICLOUD_ACCESS_KEY=<xxx> ALICLOUD_SECRET_KEY=<yyy> ALICLOUD_SECURITY_TOKEN=<zzz> ALICLOUD_REGION=<region>
```

# Provision and Consume cloud resources

Please refer to [provision and consume cloud resources](https://kubevela.io/docs/end-user/components/cloud-services/provider-and-consume-cloud-services)