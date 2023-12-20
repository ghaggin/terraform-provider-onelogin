terraform {
  required_providers {
    onelogin = {
        source = "github.com/ghaggin/onelogin"
    }
  }
}

provider "onelogin" {
    client_id = "abcd"
    client_secret = "1234"
    url = "1234"
}

data "onelogin_user" "test_user1" {}
