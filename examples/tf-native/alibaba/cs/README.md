

# A NAT gateway should be created first.

If using `new_nat_gateway       = true`, an error will occur.

```
│ Error: [ERROR] terraform-provider-alicloud/alicloud/resource_alicloud_nat_gateway.go:241: Resource alicloud_nat_gateway CreateNatGateway Failed!!! [SDK alibaba-cloud-sdk-go ERROR]:
│ SDKError:
│    Code: OperationFailed.NormalInventoryNotEnough
│    Message: code: 400, Standard NAT gateways are no longer offered. You can create enhanced NAT gateways and set the correct natType. request id: A1D6C6D2-9B65-4013-B802-B6D4649E71F3
│    Data: {"Code":"OperationFailed.NormalInventoryNotEnough","HostId":"vpc.aliyuncs.com","Message":"Standard NAT gateways are no longer offered. You can create enhanced NAT gateways and set the correct natType.","Recommend":"https://error-center.aliyun.com/status/search?Keyword=OperationFailed.NormalInventoryNotEnough\u0026source=PopGw","RequestId":"A1D6C6D2-9B65-4013-B802-B6D4649E71F3"}

```

# All necessary variables are as below

```
    new_nat_gateway: true
    vpc_name: "tf-k8s-vpc-poc"
    vpc_cidr: "10.0.0.0/8"
    vswitch_name_prefix: "tf-k8s-vsw-poc"
    vswitch_cidrs: [ "10.1.0.0/16", "10.2.0.0/16", "10.3.0.0/16" ]
    master_instance_types: [ "ecs.n4.xlarge","ecs.n1.large","ecs.sn1.large", "ecs.s6-c1m2.xlarge","ecs.c6e.xlarge" ]
    worker_instance_types: [ "ecs.c4.xlarge","ecs.c6e.xlarge","ecs.n4.xlarge","ecs.n1.large","ecs.sn1.large", "ecs.s6-c1m2.xlarge" ]
    k8s_pod_cidr: "192.168.5.0/24"
    k8s_service_cidr: "192.168.2.0/24"
    k8s_worker_number: 2
    cpu_core_count: 4
    memory_size: 8
    zone_id: "cn-beijing-a"
```