package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/portainer/terraform-provider-portainer/internal"
)

func main() {
	var debugMode bool
	flag.BoolVar(&debugMode, "debuggable", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	server, err := internal.NewProviderServer(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var serveOpts []tf5server.ServeOpt
	if debugMode {
		serveOpts = append(serveOpts, tf5server.WithManagedDebug())
	}
	if err := tf5server.Serve("registry.terraform.io/portainer/portainer", func() tfprotov5.ProviderServer { return server }, serveOpts...); err != nil {
		log.Fatal(err)
	}
}
