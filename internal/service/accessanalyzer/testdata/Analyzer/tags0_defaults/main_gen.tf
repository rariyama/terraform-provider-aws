# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

provider "aws" {
  default_tags {
    tags = var.provider_tags
  }
}

resource "aws_accessanalyzer_analyzer" "test" {
  analyzer_name = var.rName

}


variable "rName" {
  type     = string
  nullable = false
}


variable "provider_tags" {
  type     = map(string)
  nullable = false
}
