package cbgt

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func computeMD5(payload []byte) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, bytes.NewReader(payload)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// VersionReader is an interface to be implemented by the
// configuration providers who supports the verification of
// homogeneousness of the cluster before performing certain
// Key/Values updates related to the cluster status
type VersionReader interface {
	// ClusterVersion retrieves the cluster
	// compatibility information from the ns_server
	ClusterVersion() (uint64, error)
}

func CompatibilityVersion(version string) (uint64, error) {
	eVersion := uint64(1)
	xa := strings.Split(version, ".")
	if len(xa) < 2 {
		return eVersion, fmt.Errorf("invalid version")
	}

	majVersion, err := strconv.Atoi(xa[0])
	if err != nil {
		return eVersion, err
	}

	minVersion, err := strconv.Atoi(xa[1])
	if err != nil {
		return eVersion, err
	}

	return uint64(65536*majVersion + minVersion), nil
}
