// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	errEmptyFileName  = fmt.Errorf("invalid file name (empty)")
	errNotRegularFile = fmt.Errorf("not a regular file")
)

func verifyFile(fileName string) (string, error) {
	if fileName == "" {
		return "", errEmptyFileName
	}

	var absFileName string
	var fi os.FileInfo
	var err error

	absFileName, err = filepath.Abs(fileName)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}

	fileName = absFileName

	fi, err = os.Stat(fileName)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}

	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("%s: %w", fileName, errNotRegularFile)
	}

	// also try opening, to verify permissions
	// if last directory on path is not accessible to user, stat doesn't return EPERM
	f, err := os.Open(fileName)
	if err != nil {
		return "", fmt.Errorf("open file %w", err)
	}
	f.Close()

	return fileName, nil
}
