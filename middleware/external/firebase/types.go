package firebase

type CreateUserResponse struct {
	Kind         string `json:"kind,omitempty"`
	IdToken      string `json:"idToken,omitempty"`
	Email        string `json:"email,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    string `json:"expiresIn,omitempty"`
	LocalId      string `json:"localId"`
}

type SignInWithCustomTokenResponse struct {
	Kind         string `json:"kind,omitempty"`
	IdToken      string `json:"idToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    string `json:"expiresIn,omitempty"`
	IsNewUser    bool   `json:"isNewUser,omitempty"`
}
