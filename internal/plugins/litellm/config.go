/*
Copyright 2026, OpenNebula Project, OpenNebula Systems.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package litellm

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	Namespace         = "onellm-system"
	ConfigMapName     = "onellm-litellm-sync"
	MasterSecretName  = "litellm-publication-credentials"
	MasterSecretKey   = "LITELLM_MASTER_KEY"
	GatewaySecretName = "onellm-gateway-credentials"
	GatewaySecretKey  = "apiKey"
	RequestTimeout    = 10 * time.Second
	SyncInterval      = 30 * time.Second
)

// Config contains the two operator-supplied service addresses.
// Credentials use fixed Secret references and are never stored in the ConfigMap.
type Config struct {
	APIURL         string
	GatewayBaseURL string
}

func validURL(value string, authorityOnly bool) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid HTTP(S) base URL")
	}
	if authorityOnly && u.Path != "" && u.Path != "/" {
		return "", errors.New("gateway URL must contain only scheme, host and port")
	}
	return strings.TrimRight(value, "/"), nil
}
func ParseConfig(data map[string]string) (*Config, error) {
	if strings.TrimSpace(data["apiURL"]) == "" {
		return nil, nil
	}
	apiURL, err := validURL(data["apiURL"], false)
	if err != nil {
		return nil, err
	}
	gatewayURL, err := validURL(data["gatewayBaseURL"], true)
	if err != nil {
		return nil, errors.New("gatewayBaseURL must be an operator-supplied HTTP(S) authority")
	}
	return &Config{APIURL: apiURL, GatewayBaseURL: gatewayURL}, nil
}
