package cogs

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

// isValidURL tests a string to determine if it is a well-structured url or not.
func isValidURL(path string) bool {
	_, err := url.ParseRequestURI(path)
	if err != nil {
		return false
	}

	u, err := url.Parse(path)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	return true
}

func getHTTPFile(urlPath string, header http.Header, method, body string) ([]byte, error) {
	var buf bytes.Buffer

	if method == "" {
		method = "GET"
	}

	var i interface{}
	payload := new(bytes.Buffer)
	if body != "" {
		if err := json.Unmarshal([]byte(body), &i); err != nil {
			return nil, errors.Wrap(err, "getHTTPFile")
		}
		json.NewEncoder(payload).Encode(i)
	}

	request, err := http.NewRequest(method, urlPath, payload)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for key, values := range header {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Copy data from the response to standard output
	_, err = io.Copy(&buf, response.Body)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.Errorf("%q: %s returned status code of %d: %s", urlPath, method, response.StatusCode, buf.Bytes())
	}

	// handle io.Copy after status code check
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer response.Body.Close()

	return buf.Bytes(), nil
}

func parseHeader(v interface{}) (http.Header, error) {
	errMsg := "object must map to a string or array of strings"
	rawHeader, ok := v.(map[string]interface{}) // handle single string value header
	if !ok {
		return nil, errors.New(errMsg)
	}
	header := make(http.Header)
	for headerK, headerV := range rawHeader {
		switch vType := headerV.(type) {
		case string:
			header[headerK] = append(header[headerK], vType)
		case []interface{}: // go is unable to check for headerV.([]string) on initial cast
			for _, el := range vType {
				vStr, ok := el.(string)
				if !ok {
					return nil, errors.New(errMsg)
				}
				header[headerK] = append(header[headerK], vStr)

			}
		}
	}

	return header, nil
}
