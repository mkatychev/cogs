package cogs

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func getHTTPFile(urlPath string, header http.Header) ([]byte, error) {
	var buf bytes.Buffer

	request, err := http.NewRequest("GET", urlPath, nil)
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
		return nil, fmt.Errorf("Returned status code of %d", response.StatusCode)
	}
	defer response.Body.Close()

	// Copy data from the response to standard output
	if _, err = io.Copy(&buf, response.Body); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
