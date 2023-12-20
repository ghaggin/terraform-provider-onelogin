terraform {
  required_providers {
    onelogin = {
      source = "github.com/ghaggin/onelogin"
    }
  }
}

provider "onelogin" {
  client_id     = "<client-id>"
  client_secret = "<client-secret>"
  url           = "<onelogin-url>"
}

data "onelogin_user" "test_user1" {
  username = "<username>"
}
