package sdk

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) Run(
	ctx context.Context,
	workspaceName string,
	environment string,
	modelName string,
	method string,
	path string,
	headers map[string]string,
	body string,
	reqEditors ...RequestEditorFn,
) (*http.Response, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewReader([]byte(body))
	}

	req, err := NewRunRequest(c.RunServer, method, path, headers, workspaceName, environment, modelName, bodyReader)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if err := c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, err
	}

	return c.Client.Do(req)
}

func NewRunRequest(
	RunServer string,
	method string,
	path string,
	headers map[string]string,
	workspaceName string,
	environment string,
	modelName string,
	body io.Reader,
) (*http.Request, error) {
	var err error

	runURL, err := url.Parse(RunServer)
	if err != nil {
		return nil, err
	}
	if path != "" {
		path = workspaceName + "/models/" + modelName + "/" + path
	} else {
		path = workspaceName + "/models/" + modelName
	}

	queryURL, err := runURL.Parse(path + "?environment=" + environment)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, queryURL.String(), body)
	if err != nil {
		return nil, err
	}

	if headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/json"
	}

	for key, value := range headers {
		req.Header.Add(key, value)
	}

	return req, nil
}
