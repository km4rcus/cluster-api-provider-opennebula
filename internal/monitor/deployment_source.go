/*
Copyright 2026, OpenNebula Project, OpenNebula Systems.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package monitor

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// Deployment contains the lifecycle decision already made by the native monitor.
// Every observed deployment is included, including those not ready for inference.
type Deployment struct {
	ID        int
	Name      string
	Ready     bool
	URL       string
	ModelName string
}

// DeploymentSource is the read-only extension seam for observed OneLLM models.
// Labels are assumed stable for the lifetime of these managed resources.
type DeploymentSource interface {
	ModelDeployments() ([]Deployment, error)
	DeploymentChanges(context.Context) (<-chan struct{}, error)
}

// ModelDeployments returns copies from the native informer for background sync.
// An absent optional API or an uninitialized/stopped monitor is not an empty list.
func (m *ModelDeploymentMonitor) ModelDeployments() ([]Deployment, error) {
	if !m.ready.Load() || !m.resources.HasSynced() {
		return nil, fmt.Errorf("model deployment cache is not synchronized")
	}
	objects := m.resources.GetStore().List()
	models := make([]Deployment, 0, len(objects))
	for _, obj := range objects {
		model, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("unexpected model deployment cache object %T", obj)
		}
		id, event, err := modelDeploymentEvent(model, false)
		if err != nil {
			return nil, err
		}
		name, _, _ := unstructured.NestedString(model.Object, "spec", "model", "name")
		models = append(models, Deployment{ID: id, Name: model.GetName(), Ready: event.Event == "inference_ready", URL: event.Payload.URL, ModelName: name})
	}
	return models, nil
}

// DeploymentChanges coalesces cache changes without delaying native callbacks.
// The handler belongs to this subscription and is removed when ctx is cancelled.
func (m *ModelDeploymentMonitor) DeploymentChanges(ctx context.Context) (<-chan struct{}, error) {
	changes := make(chan struct{}, 1)
	notify := func() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		select {
		case changes <- struct{}{}:
		default:
		}
	}
	handler, err := m.resources.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { notify() },
		UpdateFunc: func(any, any) { notify() },
		DeleteFunc: func(any) { notify() },
	})
	if err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		_ = m.resources.RemoveEventHandler(handler)
	}()
	return changes, nil
}
