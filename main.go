package main

import (
	"flag"

	"github.com/portainer/terraform-provider-portainer/internal"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	var debugMode bool
	flag.BoolVar(&debugMode, "debuggable", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: internal.Provider,
		Debug:        debugMode,
		ProviderAddr: "registry.terraform.io/portainer/portainer",
	})
}
