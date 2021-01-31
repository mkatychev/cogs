package cogs

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
)

// isValidUrl tests a string to determine if it is a well-structured url or not.
func isValidUrl(path string) bool {
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

func getHTTPFile(urlPath string) ([]byte, error) {
	var buf bytes.Buffer

	response, err := http.Get(urlPath) //use package "net/http"
	if err != nil {
	}

	defer response.Body.Close()

	// Copy data from the response to standard output
	if _, err = io.Copy(&buf, response.Body); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
