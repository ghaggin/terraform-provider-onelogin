# OneLogin Terraform Provider

This repository is a [Terraform](https://www.terraform.io) provider for OneLogin. It is build using the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) and will support all resources that are supported by the OneLogin Admin Api.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.20

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install .
```

Then commit the changes to `go.mod` and `go.sum`.

## Local Provider Override

Update `~/.terraformrc` to run the provider from the locally installed version
```
provider_installation {

  dev_overrides {
      "github.com/ghaggin/onelogin" = "/Users/glen.haggin/go/bin"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```
