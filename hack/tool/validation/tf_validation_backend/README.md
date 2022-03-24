# tf_validation_backend

This module shows how to fetch the backend block from a Terraform configuration file.

## How to use

You can run the `tf_validation_backend.go` to fetch the backend name from a `.tf` file.

```bash
go run tf_validation_backend.go ${path_to_your_tf_file}
```

## What's more

Actually, the function `fetchBackendName` in `tf_validation_backend.go` is just a simple demo. You can extend it to get more information about the backend configuration. As you can see, all the attributes and blocks in the backend block are parsed into the `Remain` field which type is [hcl.Body](https://pkg.go.dev/github.com/hashicorp/hcl/v2@v2.11.1?utm_source=gopls#Body). We can get all the information by accessing the fields of the [hcl.Body](https://pkg.go.dev/github.com/hashicorp/hcl/v2@v2.11.1?utm_source=gopls#Body).

For example, the following code prints all the keys of the attributes of the backend block.

```go
attrsMap, _ := config.Terraform.Backend.Remain.JustAttributes()
for k := range attrsMap {
    fmt.Println(k)
}
```