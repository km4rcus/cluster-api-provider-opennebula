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
	"errors"

	"github.com/OpenNebula/cluster-api-provider-opennebula/internal/monitor"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Inputs is private to the publication path. Never log or expose its credentials.
type Inputs struct {
	Config                *Config
	MasterKey, GatewayKey string
}
type Snapshot struct {
	ClusterID int
	Models    []monitor.Deployment
}

type Source struct {
	kube      kubernetes.Interface
	models    monitor.DeploymentSource
	clusterID int
}

func NewSource(kube kubernetes.Interface, models monitor.DeploymentSource, clusterID int) *Source {
	return &Source{kube: kube, models: models, clusterID: clusterID}
}
func (s *Source) ReadInputs(ctx context.Context) (Inputs, error) {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	in := Inputs{}
	cm, e := s.kube.CoreV1().ConfigMaps(Namespace).Get(ctx, ConfigMapName, meta.GetOptions{})
	if apierrors.IsNotFound(e) {
		return in, nil
	}
	if e != nil {
		return in, errors.New("publication configuration unavailable")
	}
	in.Config, e = ParseConfig(cm.Data)
	if e != nil || in.Config == nil {
		return in, e
	}
	master, e := s.kube.CoreV1().Secrets(Namespace).Get(ctx, MasterSecretName, meta.GetOptions{})
	if e != nil {
		return Inputs{}, errors.New("publication Secret unavailable")
	}
	in.MasterKey = string(master.Data[MasterSecretKey])
	gateway, e := s.kube.CoreV1().Secrets(Namespace).Get(ctx, GatewaySecretName, meta.GetOptions{})
	if e != nil {
		return Inputs{}, errors.New("Gateway Secret unavailable")
	}
	in.GatewayKey = string(gateway.Data[GatewaySecretKey])
	if in.MasterKey == "" || in.GatewayKey == "" {
		return Inputs{}, errors.New("publication credential is empty")
	}

	return in, nil
}

func (s *Source) Snapshot(context.Context) (Snapshot, error) {
	models, err := s.models.ModelDeployments()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{ClusterID: s.clusterID, Models: models}, nil
}

func (s *Source) DeploymentChanges(ctx context.Context) (<-chan struct{}, error) {
	return s.models.DeploymentChanges(ctx)
}
