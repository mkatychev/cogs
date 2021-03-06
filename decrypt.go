package cogs

import (
	"fmt"

	"go.mozilla.org/sops/v3/decrypt"
)

func decryptFile(filePath string) ([]byte, error) {
	sec, err := decrypt.File(filePath, "")
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt file %s: %w", filePath, err)
	}
	return sec, nil
}
