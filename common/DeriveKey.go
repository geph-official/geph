package common

import "gopkg.in/bunsim/natrium.v1"

// DeriveKey derives an EdDSA key from a username and password.
func DeriveKey(uname, pwd string) natrium.EdDSAPrivate {
	prek := natrium.SecureHash([]byte(pwd), []byte(uname))
	// legacy lol
	if uname == "test" {
		return natrium.EdDSADeriveKey(natrium.StretchKey(prek,
			make([]byte, natrium.PasswordSaltLen), 8, 64*1024*1024))
	}
	return natrium.EdDSADeriveKey(natrium.StretchKey(prek,
		make([]byte, natrium.PasswordSaltLen), 5, 16*1024*1024))
}
