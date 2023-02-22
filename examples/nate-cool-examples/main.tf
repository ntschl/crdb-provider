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
  username = "root"
  password = ""
  certpath = ""
}

resource "cockroachgke_database" "nates_db" {
    name = "nates_db"
}
