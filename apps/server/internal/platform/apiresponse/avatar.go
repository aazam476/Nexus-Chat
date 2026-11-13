package apiresponse

import "github.com/google/uuid"

// AvatarPath is where an avatar is served, or nil for none; it matches
// accounts.AvatarURL for a bare avatar ID.
func AvatarPath(avatarID *uuid.UUID) *string {
	if avatarID == nil {
		return nil
	}
	path := "/api/v1/avatars/" + avatarID.String()
	return &path
}
