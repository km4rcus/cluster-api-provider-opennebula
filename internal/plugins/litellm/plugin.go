/*
Copyright 2026, OpenNebula Project, OpenNebula Systems.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package litellm

import (
	"context"
	"github.com/OpenNebula/cluster-api-provider-opennebula/internal/monitor"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

type publicationSource interface {
	ReadInputs(context.Context) (Inputs, error)
	Snapshot(context.Context) (Snapshot, error)
	DeploymentChanges(context.Context) (<-chan struct{}, error)
}
type Plugin struct{ source publicationSource }

func New(kube kubernetes.Interface, models monitor.DeploymentSource, clusterID int) *Plugin {
	return &Plugin{source: NewSource(kube, models, clusterID)}
}
func (p *Plugin) sync(ctx context.Context, inputs Inputs) error {
	snapshot, err := p.source.Snapshot(ctx)
	if err != nil {
		return err
	}
	client, err := NewClient(*inputs.Config, inputs.MasterKey)
	if err != nil {
		return err
	}
	defer client.Close()
	return Reconcile(ctx, client, snapshot, *inputs.Config, inputs.GatewayKey)
}

// Run is a serial worker. The source's buffered notification coalesces changes
// during an active pass. The timer refreshes settings and retries failed work.
func (p *Plugin) Run(ctx context.Context) {
	log := ctrl.Log.WithName("litellm-publication")
	changes, err := p.source.DeploymentChanges(ctx)
	if err != nil {
		log.Error(err, "cannot subscribe to deployments")
		return
	}
	ticker := time.NewTicker(SyncInterval)
	defer ticker.Stop()
	var inputs Inputs
	refresh := true
	for {
		if refresh {
			inputs, err = p.source.ReadInputs(ctx)
			if err != nil {
				inputs = Inputs{}
				log.Error(err, "configuration unavailable")
			}
			refresh = false
		}
		if inputs.Config != nil {
			if err := p.sync(ctx, inputs); err != nil && ctx.Err() == nil {
				log.Error(err, "synchronization failed; will retry")
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh = true
		case <-changes:
		}
	}
}
