package main

import (
	"github.com/zeebe-io/zeebe/clients/go/pkg/zbc"
	"zeebe/config"
	"zeebe/controller"
	"zeebe/route"
)

var zeebeClient zbc.Client

func main() {
	config.Connect()
	controller.DeployWorkflow()
	route.HandleRequests()
}


