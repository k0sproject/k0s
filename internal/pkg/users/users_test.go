package users

import (
	"testing"
)

func TestGetUID(t *testing.T) {
	uid, err := GetUID("root")
	if err != nil {
		t.Errorf("failed to find uid for root: %v", err)
	}
	if uid != 0 {
		t.Errorf("root uid is not 0. It is %d", uid)
	}

	uid, err = GetUID("some-non-existing-user")
	if err == nil {
		t.Errorf("unexpedly got uid for some-non-existing-user: %d", uid)
	}

}
