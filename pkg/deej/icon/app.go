package icon

import _ "embed"

// AppLogo is shared by the tray and the executable's Windows resources.
//
//go:embed app.ico
var AppLogo []byte

//go:embed settings-red.ico
var SettingsRed []byte

//go:embed refresh-sessions-red.ico
var RefreshSessionsRed []byte
