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

