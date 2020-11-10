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
		name, err := os.Hostname()
		if err != nil {
			return "", err
		}
		sum := md5.Sum([]byte(name))
		id = hex.EncodeToString(sum[:])
	}
	return id, err
}
