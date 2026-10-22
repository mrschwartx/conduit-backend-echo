package profiles

// ProfileDTO matches the contract "Profile" schema. bio and image are always
// present (null when not set).
type ProfileDTO struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

// ProfileResponse is the {"profile": ...} envelope.
type ProfileResponse struct {
	Profile ProfileDTO `json:"profile"`
}

// ToDTO converts a domain profile to its wire form. Exported for reuse by the
// articles and comments modules.
func ToDTO(p Profile) ProfileDTO {
	return ProfileDTO{Username: p.Username, Bio: p.Bio, Image: p.Image, Following: p.Following}
}
