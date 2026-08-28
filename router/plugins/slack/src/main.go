package main

import (
	"log"
	"os"

	service "github.com/wundergraph/cosmo/plugin/generated"
	slackclient "github.com/wundergraph/cosmo/plugin/src/slackclient"

	routerplugin "github.com/wundergraph/cosmo/router-plugin"
	"google.golang.org/grpc"
)

func main() {
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		log.Fatal("SLACK_BOT_TOKEN environment variable is not set (expected a bot user OAuth token, xoxb-...)")
	}

	svc := NewSlackService(slackclient.NewClient(token))

	pl, err := routerplugin.NewRouterPlugin(func(s *grpc.Server) {
		service.RegisterSlackServiceServer(s, svc)
	}, routerplugin.WithTracing())

	if err != nil {
		log.Fatalf("failed to create router plugin: %v", err)
	}

	pl.Serve()
}
