terraform {
  required_providers {
    cockroachgke = {
      source = "terraform.local/local/cockroachgke"
      version = "1.0.0"
    }
  }
}

provider "cockroachgke" {
  host     = "localhost"
  username = "nate"
  password = "nate"
  certpath = "/users/nate/certs/ca.crt"
}
