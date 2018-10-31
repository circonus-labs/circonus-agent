// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package config

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func verifyFile(fileName string) (string, error) {
	if fileName == "" {
		return "", errors.New("invalid file name (empty)")
	}

	var absFileName string
	var fi os.FileInfo
	var err error

	absFileName, err = filepath.Abs(fileName)
	if err != nil {
		return "", err
	}

	fileName = absFileName

	fi, err = os.Stat(fileName)
	if err != nil {
		return "", err
	}

	if !fi.Mode().IsRegular() {
		return "", errors.Errorf("%s: not a regular file", fileName)
	}

	// also try opening, to verify permissions
	// if last directory on path is not accessible to user, stat doesn't return EPERM
	f, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	f.Close()

	return fileName, nil
}
