# Contributing to Terraform Controller

Thanks for contributing to Terraform Controller!

## Prerequisites

- Go

Go version `>=1.17` is required.

- Helm Cli (optional)

Refer to [Helm official Doc](https://helm.sh/docs/intro/install/) to install `helm` Cli.

## How to start up the project

- Apply CRDs to a Kubernetes cluster

```shell
$ make install
go: creating new go.mod: module tmp
...
go get: added sigs.k8s.io/controller-tools v0.6.0
go get: added sigs.k8s.io/structured-merge-diff/v4 v4.1.0
go get: added sigs.k8s.io/yaml v1.2.0
/Users/zhouzhengxi/go/bin/controller-gen "crd:trivialVersions=true" webhook paths="./..." output:crd:artifacts:config=chart/crds
kubectl apply -f chart/crds
customresourcedefinition.apiextensions.k8s.io/configurations.terraform.core.oam.dev configured
customresourcedefinition.apiextensions.k8s.io/providers.terraform.core.oam.dev configured
```

- Run Terraform Controller

```shell
$ make run
go: creating new go.mod: module tmp
...
go get: added sigs.k8s.io/yaml v1.2.0
/Users/zhouzhengxi/go/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
/Users/zhouzhengxi/go/bin/controller-gen "crd:trivialVersions=true" webhook paths="./..." output:crd:artifacts:config=chart/crds
go run ./main.go
I0227 12:15:32.890376   38818 request.go:668] Waited for 1.001811845s due to client-side throttling, not priority and fairness, request: GET:https://47.242.126.78:6443/apis/apigatewayv2.aws.crossplane.io/v1alpha1?timeout=32s
```

## An development example for Cloud Resources Management

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

Apply [OSS Terraform Configuration](./examples/alibaba/oss/configuration_hcl_bucket.yaml) to provision an Alibaba OSS bucket.

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

Use [ossutil cli](https://www.alibabacloud.com/help/en/doc-detail/207217.htm) to check whether OSS bucket is provisioned.

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
    ...

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
