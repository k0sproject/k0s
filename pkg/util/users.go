package util

import (
	"os/user"
	"strconv"
)

// GetUid returns uid of given username and logs a warning if its missing
func GetUid(name string) (int, error) {
	entry, err := user.Lookup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(entry.Uid)
}

// GetGid returns gid of given groupname and logs a warning if its missing
func GetGid(name string) (int, error) {
	entry, err := user.LookupGroup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(entry.Gid)
}
