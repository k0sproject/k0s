package util

import "crypto/rand"

var letters = "abcdefghijklmnopqrstuvwxyz0123456789"

func RandomString(length int) string {

	bytes := make([]byte, length)
	rand.Read(bytes)

	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}
	return string(bytes)
}
