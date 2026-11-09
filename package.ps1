param(
    [string]$Version = "1.0.3",
    [string]$OutputDir = "dist"
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
Push-Location $root
try {
    & (Join-Path $root 'build.ps1') -Version "v$Version"
    if ($LASTEXITCODE -ne 0) { throw 'No se pudo compilar AudioDeejSwitch.' }

    $outPath = Join-Path $root $OutputDir
    New-Item -ItemType Directory -Force -Path $outPath | Out-Null
    Copy-Item -LiteralPath (Join-Path $root 'AudioDeejSwitch.exe') -Destination (Join-Path $outPath 'AudioDeejSwitch.exe') -Force

    $issPath = Join-Path $root 'packaging\AudioDeejSwitch.iss'
    if (-not (Test-Path -LiteralPath $issPath)) { throw "No se encontro $issPath" }

    $isccCandidates = @(
        'C:\Program Files (x86)\Inno Setup 6\ISCC.exe',
        'C:\Program Files\Inno Setup 6\ISCC.exe'
    )
    $iscc = $isccCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    if (-not $iscc) { throw 'No se encontro Inno Setup 6 (ISCC.exe).' }

    & $iscc "/DMyAppVersion=$Version" "/DSourceDir=$outPath" "/DOutputDir=$outPath" $issPath
    if ($LASTEXITCODE -ne 0) { throw 'Inno Setup fallo.' }
    Write-Output "Instalador generado en $outPath"
}
finally {
    Pop-Location
}
