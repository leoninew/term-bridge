package browser

type RuntimeConfig struct {
	Local RuntimeLocalConfig `json:"local"`
	Cloud RuntimeCloudConfig `json:"cloud"`
}

type RuntimeLocalConfig struct {
	Mode       string           `json:"mode"`
	PublicUrl  string           `json:"publicUrl"`
	ApiBaseUrl string           `json:"apiBaseUrl"`
	CloudOAuth CloudOAuthConfig `json:"cloudOAuth"`
}

type CloudOAuthConfig struct {
	ClientId    string   `json:"clientId"`
	RedirectUrl string   `json:"redirectUrl"`
	Scopes      []string `json:"scopes"`
}

type RuntimeCloudConfig struct {
	PublicUrl  string `json:"publicUrl"`
	ApiBaseUrl string `json:"apiBaseUrl"`
}
