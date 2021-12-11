package config

// PublicKey is a public SSH key.
type PublicKey struct {
	Name string `json:"name"`
	// Key is the public key textual representation. Begins with Begins with
	// 'ssh-rsa', 'ssh-dss', 'ssh-ed25519', 'ecdsa-sha2-nistp256',
	// 'ecdsa-sha2-nistp384', or 'ecdsa-sha2-nistp521'.
	Key string `json:"key"`
}
