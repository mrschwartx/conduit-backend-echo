// Package profiles implements public user profiles and the follow relationship.
package profiles

// Profile is the public view of a user relative to the caller.
type Profile struct {
	ID        int64
	Username  string
	Bio       *string
	Image     *string
	Following bool
}
