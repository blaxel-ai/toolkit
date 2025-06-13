package sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) Run(
	ctx context.Context,
	workspaceName string,
	resourceType string,
	resourceName string,
	method string,
	path string,
	headers map[string]string,
	params []string,
	body string,
	debug bool,
	local bool,
	reqEditors ...RequestEditorFn,
) (*http.Response, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewReader([]byte(body))
	}

	req, err := NewRunRequest(c.RunServer, method, path, headers, params, local, debug, workspaceName, resourceType, resourceName, bodyReader)
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
	params []string,
	local bool,
	debug bool,
	workspaceName string,
	resourceType string,
	resourceName string,
	body io.Reader,
) (*http.Request, error) {
	var err error
	var runURL *url.URL
	if local {
		runURL, err = url.Parse("http://localhost:1338")
	} else {
		runURL, err = url.Parse(RunServer)
	}
	if err != nil {
		return nil, err
	}

	path = strings.TrimPrefix(path, "/")
	if local {
		path = "" + path
	} else if path != "" {
		path = fmt.Sprintf("%s/%s/%s/%s", workspaceName, Pluralize(resourceType), resourceName, path)
	} else {
		path = fmt.Sprintf("%s/%s/%s", workspaceName, Pluralize(resourceType), resourceName)
	}

	if debug {
		params = append(params, "debug=true")
	}

	queryURL, err := runURL.Parse(path + "?" + strings.Join(params, "&"))
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
