package cogs

import (
	"fmt"

	"go.mozilla.org/sops/v3/decrypt"
)

func decryptFile(filePath string) ([]byte, error) {
	sec, err := decrypt.File(filePath, "yaml")
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt file: %s", err)
	}
	return sec, nil
}
