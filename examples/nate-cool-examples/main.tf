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

resource "cockroachgke_database" "nate_db" {
  name = "nate_db"
}

resource "cockroachgke_user" "nate_user" {
  username = "nate"
  password = "natepw"
  database = cockroachgke_database.nate_db.name
}