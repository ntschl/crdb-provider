package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

func NewChangefeedResource() resource.Resource {
	return &DatabaseResource{}
}
