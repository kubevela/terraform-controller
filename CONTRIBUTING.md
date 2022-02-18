# Contributing to Terraform Controller

## Prerequisites

- Go

Go version `>=1.17` is required.

- Helm Cli

Refer to [Helm official Doc](https://helm.sh/docs/intro/install/) to install `helm` Cli.

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

## Cloud Resources Management

Let's take Alibaba Cloud as an example.

### Apply Provider credentials

```shell
$ export ALICLOUD_ACCESS_KEY=xxx; export ALICLOUD_SECRET_KEY=yyy
```

If you'd like to use Alicloud Security Token Service, also export `ALICLOUD_SECURITY_TOKEN`.
```shell
$ export ALICLOUD_SECURITY_TOKEN=zzz
```

```
$ make alibaba
```

### Apply Terraform Configuration

Apply Terraform configuration [configuration_hcl_oss.yaml](./examples/alibaba/oss/configuration_hcl_bucket.yaml) (JSON configuration [configuration_oss.yaml](./examples/alibaba/oss/configuration_json_bucket.yaml) is also supported) to provision an Alibaba OSS bucket.

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
