/*
Copyright 2026, OpenNebula Project, OpenNebula Systems.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package litellm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type Model struct {
	ID   string
	Info map[string]interface{}
}
type APIError struct{ Status int }

func (e *APIError) Error() string { return fmt.Sprintf("LiteLLM HTTP %d", e.Status) }

type Client struct {
	url, key string
	http     *http.Client
}

func NewClient(cfg Config, masterKey string) (*Client, error) {
	base, e := validURL(cfg.APIURL, false)
	if e != nil {
		return nil, e
	}
	if masterKey == "" {
		return nil, errors.New("LiteLLM master key is empty")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Client{url: base, key: masterKey, http: &http.Client{Transport: transport, Timeout: RequestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Close() { c.http.CloseIdleConnections() }
func (c *Client) request(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
	var data []byte
	var e error
	if body != nil {
		data, e = json.Marshal(body)
		if e != nil {
			return nil, errors.New("invalid model management payload")
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, c.url+path, bytes.NewReader(data))
	if e != nil {
		return nil, errors.New("invalid management request")
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	res, e := c.http.Do(req)
	if e != nil {
		return nil, errors.New("LiteLLM management request failed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &APIError{res.StatusCode}
	}
	raw, e := io.ReadAll(res.Body)
	if e != nil {
		return nil, errors.New("invalid management response")
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var out map[string]interface{}
	if json.Unmarshal(raw, &out) != nil || out == nil {
		return nil, errors.New("invalid management response")
	}
	return out, nil
}
func (c *Client) List(ctx context.Context) ([]Model, error) {
	response, e := c.request(ctx, "GET", "/model/info", nil)
	if e != nil {
		return nil, e
	}
	rows, ok := response["data"].([]interface{})
	if !ok {
		return nil, errors.New("incomplete LiteLLM model inventory")
	}
	models := make([]Model, 0, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]interface{})
		if !ok {
			return nil, errors.New("invalid native model row")
		}
		info, ok := row["model_info"].(map[string]interface{})
		if !ok {
			return nil, errors.New("native model identity unavailable")
		}
		id := textField(info, "id")
		if id == "" {
			return nil, errors.New("native model identity unavailable")
		}
		models = append(models, Model{ID: id, Info: info})
	}
	return models, nil
}
func (c *Client) Create(ctx context.Context, body map[string]interface{}) error {
	body["model_info"].(map[string]interface{})["mode"] = "chat"
	_, err := c.request(ctx, "POST", "/model/new", body)
	return err
}
func (c *Client) Patch(ctx context.Context, model Model, body map[string]interface{}) error {
	_, err := c.request(ctx, "PATCH", "/model/"+model.ID+"/update", body)
	return err
}
func (c *Client) Delete(ctx context.Context, m Model) error {
	_, e := c.request(ctx, "POST", "/model/delete", map[string]string{"id": m.ID})
	return e
}

func textField(fields map[string]interface{}, key string) string {
	value, _ := fields[key].(string)
	return value
}
