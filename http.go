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

func getHTTPFile(urlPath string, header http.Header, method string, body interface{}) ([]byte, error) {
	var buf bytes.Buffer

	if method == "" {
		method = "GET"
	}

	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(body)

	request, err := http.NewRequest(method, urlPath, payload)
	if err != nil {
		return nil, err
	}
	for key, values := range header {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	} else if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.Errorf("%q: %s returned status code of %d", urlPath, method, response.StatusCode)
	}
	defer response.Body.Close()

	// Copy data from the response to standard output
	if _, err = io.Copy(&buf, response.Body); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
