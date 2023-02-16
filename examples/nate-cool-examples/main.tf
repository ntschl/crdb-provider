terraform {
  required_providers {
    cockroachgke = {
      
    }
  }
}

provider "cockroachgke" {
  host     = "localhost"
  username = "root"
  password = ""
  certpath = ""
}

data "cockroachgke_example" "edu" {}
