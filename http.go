package cogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/pkg/errors"
)

// DefaultMethod uses GET for the default request type
var DefaultMethod string = http.MethodGet

// isValidURL tests a string to determine if it is a well-structured url or not.
func isValidURL(path string) bool {
	if _, err := url.ParseRequestURI(path); err != nil {
		return false
	}

	if u, err := url.Parse(path); err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	return true
}

func getHTTPFile(urlPath string, header http.Header, method, body string) ([]byte, error) {
	var buf bytes.Buffer

	if method == "" {
		method = DefaultMethod
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
	var rawHeader map[string]interface{} // handle single string value header
	header := make(http.Header)
	errMsg := "object must map to a string or array of strings"

	switch t := v.(type) {
	case map[string]interface{}:
		rawHeader = t
	case http.Header:
		for k, vals := range t {
			cannonicalKey := textproto.CanonicalMIMEHeaderKey(k)
			for _, v := range vals {
				header[cannonicalKey] = append(header[cannonicalKey], v)
			}
		}
		return header, nil
	default:
		return nil, fmt.Errorf("%s: %T", errMsg, v)
	}

	for rawK, rawV := range rawHeader {
		key := textproto.CanonicalMIMEHeaderKey(rawK)
		switch vType := rawV.(type) {
		case string:
			header[key] = append(header[key], vType)
		case []interface{}: // go is unable to check for headerV.([]string) on initial cast
			for _, el := range vType {
				vStr, ok := el.(string)
				if !ok {
					return nil, errors.New(errMsg)
				}
				header[key] = append(header[key], vStr)
			}
		}
	}

	return header, nil
}
