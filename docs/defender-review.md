# Revisión de la detección de Microsoft Defender

## Estado de la investigación — 26 de octubre de 2026

Detección comunicada por el usuario: `Trojan:Win32/Bearfoos.B!ml` sobre
`AudioDeejSwitch.exe`. Pendiente de confirmación o rectificación por Microsoft.

### Archivo original comprobado en este entorno

```text
SHA-256: 4FCF876AD3C73DB2C7184A1F8512171250515FF24BBE8B3DD9280F647F640ABF
Go: go1.26.10 windows/amd64
Firma Authenticode: NotSigned
```

Este hash identifica la versión anterior al cambio de notificaciones. Se conservó
una copia en la carpeta temporal de herramientas antes de sustituir el ejecutable.
Coincide con el obtenido al principio de esta investigación. No se ha
comparado todavía con el hash del archivo que Defender detectó en el equipo del
usuario: una ruta o un nombre iguales no bastan para establecer esa identidad.

### Comprobaciones realizadas

- `go version -m AudioDeejSwitch.exe`: se pudieron leer los metadatos de Go y las
  dependencias del binario. Corresponden al módulo de deej y a las versiones
  declaradas en el proyecto; esto no prueba que todos sus bytes procedan del
  código fuente actual.
- `go mod verify`: resultado `all modules verified`. Comprueba la integridad de
  los módulos presentes en la caché; no es un análisis de vulnerabilidades ni
  certifica que las dependencias sean benignas.
- Revisión dirigida del arranque, atajos, captura de combinaciones, cambio de
  salida, apertura de configuración y ruta de notificaciones.
- `Update-MpSignature`: las firmas locales pasaron de `1.461.197.0` a
  `1.461.204.0`; motor `1.1.26090.9`.
- Análisis personalizado con `Start-MpScan`: los eventos 1000 y 1001 registraron
  el inicio y la finalización del análisis del ejecutable.
- Análisis adicional con la herramienta oficial de Defender:

  ```powershell
  & 'C:\ProgramData\Microsoft\Windows Defender\Platform\4.18.26090.9-0\MpCmdRun.exe' -Scan -ScanType 3 -File '.\AudioDeejSwitch.exe' -DisableRemediation
  ```

  Salida: `Scanning ... AudioDeejSwitch.exe found no threats.` Código de salida:
  `0`. `-DisableRemediation` hace que ese análisis informe sin aplicar medidas
  correctivas; no desactiva la protección en tiempo real.
- Las consultas locales `Get-MpThreat` y `Get-MpThreatDetection` no devolvieron
  amenazas. La protección en tiempo real está habilitada. No se pudieron consultar
  las exclusiones porque el proceso no tiene permisos de administrador, por lo
  que este resultado no permite descartar una exclusión preexistente.

El resultado local no reproduce el aviso del usuario y no confirma un falso
positivo. No se ha ejecutado el programa para observar su comportamiento.

### Comportamiento relevante de las dependencias

En la versión original, `pkg/deej/notify.go` llamaba a `beeep.Notify`.
En las versiones fijadas:

- `github.com/gen2brain/beeep`, revisión `13046a26d502`, ejecuta
  `powershell Get-StartApps` durante la inicialización en Windows 10.
- `github.com/go-toast/toast`, revisión `01e6764cf0a4`, escribe un archivo `.ps1`
  temporal para cada notificación y ejecuta
  `PowerShell -ExecutionPolicy Bypass -File <archivo>` con la ventana oculta.

Se comprobó este código tanto en las fuentes remotas de esas revisiones como en
los módulos locales, cuya integridad pasó `go mod verify`. Es el mecanismo de
notificaciones de estas librerías; no hay evidencia que lo vincule específicamente
con la detección Bearfoos. El cambio a notificaciones nativas descrito debajo
elimina ese mecanismo de la versión nueva; no demuestra que causara esta alerta.

## Cambio implementado y compilación nueva

Se sustituyó la ruta de notificaciones de Windows por `Shell_NotifyIcon`, usando
la dependencia Win32 que ya estaba en el proyecto. Un hilo propio mantiene una
ventana oculta para recibir mensajes del sistema, solicita los avisos y retira su
icono temporal tras 15 segundos sin avisos nuevos. El host se cierra al detener
deej. La ruta de Linux conserva `beeep`.

`go list -deps` y `go version -m` confirman que el ejecutable nuevo de Windows no
incluye `github.com/gen2brain/beeep`, `github.com/go-toast/toast` ni
`github.com/tadvi/systray`. Que esas dependencias sigan en `go.mod` para Linux no
significa que se enlacen en Windows. La compilación rechaza los dos backends de
notificaciones por scripts para prevenir su reintroducción.

Se ejecutaron correctamente:

```powershell
go mod verify
go test ./... -count=1 -timeout=90s
go vet ./...
.\build.ps1 -Clean
```

Las pruebas cubren el texto Unicode, la terminación y truncación de los campos
nativos, la creación y cierre del host, el cierre repetido y el rechazo de avisos
después del cierre. También se ejecutó `TestWindowsNotifierLifecycle` con
`AUDIODEEJSWITCH_TEST_NOTIFICATIONS=1`, que solicita un aviso real mediante la API
nativa. El resultado verifica que Windows aceptó la solicitud, no su visibilidad.

`-Clean` fuerza `go build -a -mod=readonly` y el script regenera los iconos y los
recursos de Windows con `github.com/akavel/rsrc@v0.10.2`. Es una recompilación
completa local, no una compilación en una máquina limpia ni aislada.
No se ha completado un análisis de vulnerabilidades conocidas de las dependencias.

### Ejecutable nuevo

```text
Archivo: AudioDeejSwitch.exe
SHA-256: 0F5228178AE0470058E41ABC2E75E28E43163643A4ED89207E5C092DA3F88518
Go: go1.26.10 windows/amd64
Firma Authenticode: NotSigned
```

Después de actualizar Defender a las firmas `1.461.211.0`, motor `1.1.26090.9`,
se analizó el archivo nuevo con `MpCmdRun -Scan -ScanType 3 -File ...
-DisableRemediation`. Resultado: `found no threats`, código `0`, con protección
en tiempo real habilitada. Se mantiene la limitación sobre la consulta de
exclusiones del entorno. La versión anterior también pasó un análisis local:
no puede concluirse que la ausencia de detección se deba a este cambio.

Falta comprobar el nuevo archivo en el equipo que mostró la alerta. No se han
cambiado las exclusiones ni desactivado la protección de Defender.

## Envío a Microsoft realizado por el usuario

Portal oficial: <https://www.microsoft.com/en-us/wdsi/filesubmission>.
El usuario ha confirmado que realizó el envío. Está pendiente la respuesta de
Microsoft; el identificador del caso no se ha facilitado en esta sesión.
El texto de abajo corresponde al archivo original. La compilación nueva tiene
otro hash: si también es detectada, debe identificarse por separado en el caso o
en un nuevo envío, describiendo la eliminación del backend PowerShell.

Se conservan las instrucciones y el texto preparados para ese primer envío:

1. Elegir **Software developer** (o **Home customer**, si se presenta como usuario).
2. Seleccionar **Microsoft Defender Antivirus** para la versión de Windows afectada.
3. Adjuntar el ejecutable exacto que produjo el aviso, sin lanzarlo. Comparar antes
   su SHA-256 con el indicado arriba y corregir el texto si difiere.
4. Elegir **Incorrectly detected as malware/malicious** como clasificación que
   se solicita revisar, no como conclusión ya confirmada.
5. Detección: `Trojan:Win32/Bearfoos.B!ml`. Usar la versión de firmas del equipo
   que lo detectó si está disponible, diferenciándola de la comprobación local.
6. Pegar el siguiente texto, completar el formulario y guardar el ID del envío.

### Texto en inglés para el formulario

```text
Please investigate a suspected false positive affecting AudioDeejSwitch.exe,
a portable Windows audio mixer and output switcher based on the open-source
deej project (github.com/omriharel/deej). The user reports detection as
Trojan:Win32/Bearfoos.B!ml after the application had been running normally.
Downloading the executable again also triggered detection on the affected PC.

SHA-256 of the copy checked during this investigation:
4FCF876AD3C73DB2C7184A1F8512171250515FF24BBE8B3DD9280F647F640ABF

This copy is unsigned and contains Go 1.26.10 windows/amd64 build metadata.
Its intended functions are Arduino serial input, per-application volume control,
global shortcuts using RegisterHotKey, Windows audio output switching, and a
native settings window. The manifest requests asInvoker privileges.

The legacy notification dependencies beeep (13046a26d502) and go-toast/toast
(01e6764cf0a4) invoke PowerShell to show notifications. The latter writes a
temporary .ps1 and runs PowerShell -ExecutionPolicy Bypass -File with a hidden
window. Please assess whether this legitimate notification implementation is
contributing to the classification; we have not established causality.

go mod verify reported all modules verified. A local custom scan of the checked
copy using Defender engine 1.1.26090.9 and security intelligence 1.461.204.0
reported no threats (MpCmdRun exit code 0). Local exclusions could not be
inspected without administrator access, and the affected PC may use different
settings or intelligence. A fresh isolated build has not yet been validated.

Please analyze the attached file and correct its classification if appropriate.
```

Tras la respuesta de Microsoft, actualizar Defender en el equipo afectado y
repetir el análisis del mismo archivo. La firma de código para futuras versiones
puede establecer la identidad del editor, pero no garantiza evitar detecciones
de malware ni sustituye esta revisión.
