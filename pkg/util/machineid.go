package util

import (
	"crypto/md5"
	"encoding/hex"
	"os"

	"github.com/denisbrodbeck/machineid"
)

// MachineID returns protected id for the current machine
func MachineID() (string, error) {
	id, err := machineid.ProtectedID("k0sproject-k0s")
	if err != nil {
		return MachineIDFromHostname()
	}
	return id, err
}

// MachineIDFromHostname generates a machine id hash from hostname
func MachineIDFromHostname() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		return "", err
	}
	sum := md5.Sum([]byte(name))
	return hex.EncodeToString(sum[:]), nil
}
