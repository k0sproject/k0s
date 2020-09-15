package util

import (
	"os/user"
	"strconv"
)

// GetUID returns uid of given username and logs a warning if its missing
func GetUID(name string) (int, error) {
	entry, err := user.Lookup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(entry.Uid)
}

// GetGID returns gid of given groupname and logs a warning if its missing
func GetGID(name string) (int, error) {
	entry, err := user.LookupGroup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(entry.Gid)
}
