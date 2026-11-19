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
	"encoding/json"
	"net/url"
	"strings"
)

const OwnerField = "onellm_publication"

// Ownership identifies the publishing cluster and OpenNebula deployment.
type Ownership struct {
	ClusterID    int `json:"cluster_id"`
	DeploymentID int `json:"deployment_id"`
}

func ownership(model Model) (Ownership, bool) {
	value, ok := model.Info[OwnerField]
	if !ok {
		return Ownership{}, false
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return Ownership{}, false
	}
	var owner Ownership
	err = json.Unmarshal(raw, &owner)
	return owner, err == nil
}

// Reconcile relies on the monitor for lifecycle decisions. Present, non-ready
// deployments keep their registrations; only missing deployments are removed.
func Reconcile(ctx context.Context, client *Client, snapshot Snapshot, config Config, gatewayKey string) error {
	rows, err := client.List(ctx)
	if err != nil {
		return err
	}
	owned := make(map[int]Model)
	for _, row := range rows {
		owner, ok := ownership(row)
		if ok && owner.ClusterID == snapshot.ClusterID {
			owned[owner.DeploymentID] = row
		}
	}
	present := make(map[int]bool)
	for _, model := range snapshot.Models {
		present[model.ID] = true
		if !model.Ready {
			continue
		}
		u, err := url.Parse(model.URL)
		if err != nil {
			return err
		}
		path := strings.TrimRight(u.Path, "/")
		path = strings.TrimSuffix(path, "/chat/completions")
		if !strings.HasSuffix(path, "/v1") {
			path += "/v1"
		}
		registration := map[string]interface{}{
			"model_name":     model.Name,
			"litellm_params": map[string]interface{}{"model": "openai/" + model.ModelName, "api_base": config.GatewayBaseURL + path, "api_key": gatewayKey},
			"model_info":     map[string]interface{}{OwnerField: Ownership{ClusterID: snapshot.ClusterID, DeploymentID: model.ID}},
		}
		if row, ok := owned[model.ID]; ok {
			err = client.Patch(ctx, row, registration)
		} else {
			err = client.Create(ctx, registration)
		}
		if err != nil {
			return err
		}
	}
	for id, row := range owned {
		if !present[id] {
			if err := client.Delete(ctx, row); err != nil {
				return err
			}
		}
	}
	return nil
}
