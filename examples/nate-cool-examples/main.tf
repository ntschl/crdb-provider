terraform {
  required_providers {
    cockroachgke = {
      source  = "terraform.local/local/cockroachgke"
      version = "1.0.0"
    }
  }
}

provider "cockroachgke" {
  host     = "punchy-warthog-3928.g8z.cockroachlabs.cloud"
  username = "nate"
  password = "2pJmQ5pl0fD3-EVWob9UwA"
  certpath = ""
}

resource "cockroachgke_database" "nate_db" {
  name = "nate_db"
}
