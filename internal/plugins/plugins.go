/*
Copyright 2026, OpenNebula Project, OpenNebula Systems.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package plugins attaches optional consumers to native monitor observations.
package plugins

import (
	"context"
	"github.com/OpenNebula/cluster-api-provider-opennebula/internal/monitor"
	"github.com/OpenNebula/cluster-api-provider-opennebula/internal/plugins/litellm"
	"k8s.io/client-go/kubernetes"
)

// Start returns the explicit stop/join function for optional background work.
// Plug-ins do not participate in the native Manager's readiness or error path.
func Start(ctx context.Context, kube kubernetes.Interface, models monitor.DeploymentSource, clusterID int) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	plugin := litellm.New(kube, models, clusterID)
	go func() { defer close(done); plugin.Run(ctx) }()
	return func() { cancel(); <-done }
}
