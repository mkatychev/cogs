package cogs

import (
	"fmt"
	"net/http"

	"go.mozilla.org/sops/v3/decrypt"
)

func decryptFile(filePath string) ([]byte, error) {
	sec, err := decrypt.File(filePath, "")
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt file %s: %w", filePath, err)
	}
	return sec, nil
}

func decryptHTTPFile(urlPath string, header http.Header) ([]byte, error) {
	encData, err := getHTTPFile(urlPath, header)
	if err != nil {
		return nil, err
	}
	format := FormatForPath(urlPath)
	return decrypt.Data(encData, string(format))
}
