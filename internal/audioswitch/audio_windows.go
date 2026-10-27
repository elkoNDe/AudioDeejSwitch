package audioswitch

import (
	"fmt"
	"sort"
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	wca "github.com/moutend/go-wca"
)

type Device struct {
	ID, Name string
	Default  bool
}

func deviceID(d *wca.IMMDevice) (string, error) {
	var p *uint16
	// The upstream go-wca GetId wrapper stores the pointer in uint32, which
	// truncates it on 64-bit Windows. Call this slot with a pointer-sized value.
	hr, _, _ := syscall.Syscall(d.VTable().GetId, 2, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&p)), 0)
	if int32(hr) < 0 {
		return "", ole.NewError(hr)
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(p)))
	return ole.LpOleStrToString(p), nil
}

// Caller owns a COM apartment on a locked OS thread.
func Devices() ([]Device, error) {
	var e *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &e); err != nil {
		return nil, err
	}
	defer e.Release()
	defaultID := ""
	var def *wca.IMMDevice
	if e.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &def) == nil {
		defaultID, _ = deviceID(def)
		def.Release()
	}
	var collection *wca.IMMDeviceCollection
	if err := e.EnumAudioEndpoints(wca.ERender, wca.DEVICE_STATE_ACTIVE, &collection); err != nil {
		return nil, err
	}
	defer collection.Release()
	var count uint32
	if err := collection.GetCount(&count); err != nil {
		return nil, err
	}
	devices := []Device{}
	for i := uint32(0); i < count; i++ {
		d, err := readDevice(collection, i)
		if err != nil {
			return nil, err
		}
		d.Default = d.ID == defaultID
		devices = append(devices, d)
	}
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Name == devices[j].Name {
			return devices[i].ID < devices[j].ID
		}
		return devices[i].Name < devices[j].Name
	})
	return devices, nil
}

func readDevice(c *wca.IMMDeviceCollection, index uint32) (Device, error) {
	var endpoint *wca.IMMDevice
	if err := c.Item(index, &endpoint); err != nil {
		return Device{}, err
	}
	defer endpoint.Release()
	id, err := deviceID(endpoint)
	if err != nil {
		return Device{}, err
	}
	var store *wca.IPropertyStore
	if err := endpoint.OpenPropertyStore(wca.STGM_READ, &store); err != nil {
		return Device{}, err
	}
	defer store.Release()
	var value wca.PROPVARIANT
	if err := store.GetValue(&wca.PKEY_Device_FriendlyName, &value); err != nil {
		return Device{}, err
	}
	// This version of go-wca's String also frees the allocated string. Do not
	// call PropVariantClear afterwards: that would free the same pointer twice.
	name := value.String()
	return Device{ID: id, Name: name}, nil
}

// IPolicyConfig is the Windows interface also commonly used by audio switchers.
// Its first 13 vtable slots precede SetDefaultEndpoint (IUnknown included).
// Keep the COM pointer scoped to this thread and never pass it to deej.
func SetOutput(id string, communications bool) error {
	devices, err := Devices()
	if err != nil {
		return err
	}
	found := false
	for _, d := range devices {
		if d.ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("la salida no está conectada o está deshabilitada")
	}
	var policy *ole.IUnknown
	clsid := ole.NewGUID("{870AF99C-171D-4F9E-AF0D-E63DF40C2BC9}")
	iid := ole.NewGUID("{F8679F50-850A-41CF-9C72-430F290290C8}")
	if err := wca.CoCreateInstance(clsid, 0, wca.CLSCTX_ALL, iid, &policy); err != nil {
		return err
	}
	defer policy.Release()
	wide, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return err
	}
	vtable := *(*[14]uintptr)(unsafe.Pointer(policy.RawVTable))
	roles := []uintptr{0, 1} // console and multimedia
	if communications {
		roles = append(roles, 2)
	}
	for _, role := range roles {
		hr, _, _ := syscall.Syscall(vtable[13], 3, uintptr(unsafe.Pointer(policy)), uintptr(unsafe.Pointer(wide)), role)
		if int32(hr) < 0 {
			return fmt.Errorf("SetDefaultEndpoint (rol %d): 0x%08X; puede haberse aplicado parcialmente", role, uint32(hr))
		}
	}
	return nil
}

func Switch(target string, communications bool) (string, error) {
	devices, err := Devices()
	if err != nil {
		return "", err
	}
	if len(devices) == 0 {
		return "", fmt.Errorf("no hay salidas de audio activas")
	}
	if target == "cycle" {
		target = devices[0].ID
		for i, d := range devices {
			if d.Default {
				target = devices[(i+1)%len(devices)].ID
				break
			}
		}
	}
	for _, d := range devices {
		if d.ID == target {
			return d.Name, SetOutput(target, communications)
		}
	}
	return "", fmt.Errorf("la salida asignada está desconectada; vuelve a conectarla o cambia el atajo")
}
