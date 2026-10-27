param([switch]$Clean)

$ErrorActionPreference = 'Stop'
if (-not (Test-Path -LiteralPath './go.mod')) { throw 'Ejecuta este script desde la carpeta AudioDeejSwitch.' }
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'La integridad de las dependencias no es correcta.' }
# Keep the legacy script-based notification backends out of the Windows binary.
$dependencies = @(go list -mod=readonly -deps ./pkg/deej/cmd)
if ($LASTEXITCODE -ne 0) { throw 'No se pudieron comprobar las dependencias de Windows.' }
if ($dependencies -match '^github\.com/(gen2brain/beeep|go-toast/toast)(/|$)') {
    throw 'La compilacion de Windows incluye un backend de notificaciones basado en scripts.'
}
go run ./packaging/make-icon.go
if ($LASTEXITCODE -ne 0) { throw 'No se pudo generar el icono.' }
if (Test-Path -LiteralPath './pkg/deej/cmd/rsrc_windows.syso') { Remove-Item -LiteralPath './pkg/deej/cmd/rsrc_windows.syso' -Force }
go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -manifest ./packaging/AudioDeejSwitch.manifest.xml -ico ./assets/deej-red-black.ico -o ./pkg/deej/cmd/rsrc_windows_amd64.syso
if ($LASTEXITCODE -ne 0) { throw 'No se pudo generar los recursos de Windows.' }
$buildArgs = @('build', '-mod=readonly', '-trimpath', '-ldflags', '-H windowsgui', '-o', './AudioDeejSwitch.exe')
if ($Clean) { $buildArgs += '-a' }
$buildArgs += './pkg/deej/cmd'
& go @buildArgs
if ($LASTEXITCODE -ne 0) { throw 'No se pudo compilar AudioDeejSwitch.' }
# Explorer caches icons by executable path. Refresh after replacing the binary
# so an existing folder view or shortcut does not keep the previous logo.
if (-not ('AudioDeejSwitch.ShellIcons' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
namespace AudioDeejSwitch {
    public static class ShellIcons {
        [DllImport("shell32.dll", CharSet = CharSet.Unicode)]
        public static extern void SHChangeNotify(uint eventId, uint flags, string path, IntPtr extra);
    }
}
'@
}
$exePath = (Resolve-Path -LiteralPath './AudioDeejSwitch.exe').Path
[AudioDeejSwitch.ShellIcons]::SHChangeNotify(0x00002000, 0x00001005, $exePath, [IntPtr]::Zero)
[AudioDeejSwitch.ShellIcons]::SHChangeNotify(0x08000000, 0x00001000, $null, [IntPtr]::Zero)
'Compilado: AudioDeejSwitch.exe'
