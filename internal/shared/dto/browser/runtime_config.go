package browser

type RuntimeConfig struct {
	Version  string                 `json:"version"`
	Local    RuntimeLocalConfig     `json:"local"`
	Cloud    RuntimeCloudConfig     `json:"cloud"`
	Terminal *RuntimeTerminalConfig `json:"terminal,omitempty"`
}

type RuntimeTerminalConfig struct {
	KeepAlive RuntimeTerminalKeepAliveConfig `json:"keepAlive"`
}

type RuntimeTerminalKeepAliveConfig struct {
	MaxHotTerminals int `json:"maxHotTerminals"`
	DisposeDelayMs  int `json:"disposeDelayMs"`
}

type RuntimeLocalConfig struct {
	Mode        string           `json:"mode"`
	PublicUrl   string           `json:"publicUrl"`
	ApiBasePath string           `json:"apiBasePath"`
	CloudOAuth  CloudOAuthConfig `json:"cloudOAuth"`
}

type CloudOAuthConfig struct {
	ClientId    string   `json:"clientId"`
	RedirectUrl string   `json:"redirectUrl"`
	Scopes      []string `json:"scopes"`
}

type RuntimeCloudConfig struct {
	PublicUrl               string   `json:"publicUrl"`
	ApiBaseUrl              string   `json:"apiBaseUrl"`
	ExternalAuthProviderIds []string `json:"externalAuthProviderIds"`
}
