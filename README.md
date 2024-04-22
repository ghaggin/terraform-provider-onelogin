# OneLogin Terraform Provider

This repository is a [Terraform](https://www.terraform.io) provider for OneLogin. It is build using the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) and will support all resources that are supported by the [OneLogin Admin Api](https://developers.onelogin.com/api-docs/2/getting-started/dev-overview).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) = 1.7
- [Go](https://golang.org/doc/install) = 1.21

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

Then import the resource in any Terraform config files as
```
terraform {
  required_providers {
    onelogin = {
      source = "github.com/ghaggin/onelogin"
    }
  }
}
```

Note: you do not need to run `terraform init` when loading the plugin locally.

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, export the environment variables below, then run `make testacc`.
```shell
export CLIENT_ID='<client-id>',
export CLIENT_SECRET='<client-secret>'
export SUBDOMAIN='<subdomain>',
make testacc
```

To run the provider from a terraform config directory, setup the provider to run locally and then export the client details the same as when running acceptance tests, but with `TF_VAR_` appended.
```shell
export TF_VAR_CLIENT_ID="$CLIENT_ID"
export TF_VAR_CLIENT_SECRET="$CLIENT_SECRET"
export TF_VAR_SUBDOMAIN="$SUBDOMAIN"
```
Then add this block to your terraform config to setup the provider.
```
variable "SUBDOMAIN" {
  type = string
}

variable "CLIENT_ID" {
  type      = string
  sensitive = true
}

variable "CLIENT_SECRET" {
  type      = string
  sensitive = true
}

provider "onelogin" {
  client_id     = var.CLIENT_ID
  client_secret = var.CLIENT_SECRET
  subdomain     = var.SUBDOMAIN
}
```
