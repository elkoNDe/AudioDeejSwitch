package main

import (
	"github.com/lxn/walk"
)

func showSettingsError(err error) {
	walk.MsgBox(nil, "AudioDeejSwitch", err.Error(), walk.MsgBoxIconError)
}
