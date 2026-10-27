// Package audioswitch adds Windows output switching without changing deej's
// serial protocol or slider configuration.
package audioswitch

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

const ConfigPath = "switcher.yaml"

type Binding struct {
	Hotkey   string `yaml:"hotkey"`
	DeviceID string `yaml:"device_id"` // "cycle" means next active playback device.
	Name     string `yaml:"name"`
}

type Config struct {
	Communications bool      `yaml:"communications"`
	Bindings       []Binding `yaml:"bindings"`
}

func LoadConfig() (Config, error) {
	c := Config{}
	b, err := os.ReadFile(ConfigPath)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := yaml.UnmarshalStrict(b, &c); err != nil {
		return c, err
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	seen := map[string]bool{}
	for _, b := range c.Bindings {
		mods, key, err := ParseHotkey(b.Hotkey)
		if err != nil {
			return err
		}
		id := fmt.Sprintf("%d:%d", mods, key)
		if seen[id] {
			return fmt.Errorf("atajo repetido: %s", b.Hotkey)
		}
		seen[id] = true
		if strings.TrimSpace(b.DeviceID) == "" {
			return fmt.Errorf("falta el dispositivo de %s", b.Hotkey)
		}
	}
	return nil
}

// SaveWithBackup also detects external edits, so an old settings window cannot
// silently replace a config edited in Notepad in the meantime.
func SaveWithBackup(path string, original, data []byte) error {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if string(current) != string(original) {
		return fmt.Errorf("%s cambió fuera de esta ventana; recarga antes de guardar", path)
	}
	if err == nil {
		if err := os.WriteFile(path+".bak", current, 0600); err != nil {
			return err
		}
	}
	// Write in place: deej's watcher listens specifically for Write events.
	return os.WriteFile(path, data, 0600)
}

func ValidateDeejYAML(data []byte) error {
	var c struct {
		Mapping map[int]interface{} `yaml:"slider_mapping"`
		Port    string              `yaml:"com_port"`
		Baud    int                 `yaml:"baud_rate"`
		Invert  bool                `yaml:"invert_sliders"`
		Noise   string              `yaml:"noise_reduction"`
	}
	if err := yaml.UnmarshalStrict(data, &c); err != nil {
		return err
	}
	if len(c.Mapping) == 0 || strings.TrimSpace(c.Port) == "" || c.Baud <= 0 {
		return fmt.Errorf("se requieren slider_mapping, com_port y baud_rate positivo")
	}
	if c.Noise != "" && c.Noise != "low" && c.Noise != "default" && c.Noise != "high" {
		return fmt.Errorf("noise_reduction debe ser low, default o high")
	}
	for index, value := range c.Mapping {
		if index < 0 {
			return fmt.Errorf("el índice del slider no puede ser negativo")
		}
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("slider %d vacío", index)
			}
		case []interface{}:
			if len(v) == 0 {
				return fmt.Errorf("slider %d vacío", index)
			}
			for _, app := range v {
				name, ok := app.(string)
				if !ok || strings.TrimSpace(name) == "" {
					return fmt.Errorf("slider %d: usa nombres de aplicaciones", index)
				}
			}
		default:
			return fmt.Errorf("slider %d: usa un nombre o una lista de nombres", index)
		}
	}
	return nil
}
