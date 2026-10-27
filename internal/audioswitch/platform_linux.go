package audioswitch

import "fmt"

func Start(notify func(string, string)) (func(), func()) { return func() {}, func() {} }
func OpenSettings() error                                { return fmt.Errorf("la ventana de AudioDeejSwitch requiere Windows") }
func RunSettings() error                                 { return OpenSettings() }
