package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/stackshiftCloud/stackshift",
	})
	if err != nil {
		log.Fatal(err)
	}
}
