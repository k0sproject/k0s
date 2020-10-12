package util

import "crypto/rand"

var letters = "abcdefghijklmnopqrstuvwxyz0123456789"

// RandomString generates a random string with given length
func RandomString(length int) string {

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Not much we can do on broken system
		panic("random is broken: " + err.Error())
	}

	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}
	return string(bytes)
}
