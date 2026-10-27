# Contribuir a AudioDeejSwitch

Gracias por tomarte el tiempo de mejorar el proyecto. Esta guía resume cómo
reportar problemas, proponer cambios y preparar una contribución.

AudioDeejSwitch es un fork independiente de
[deej](https://github.com/omriharel/deej). Las propuestas y los cambios se
gestionan en este repositorio, no en el proyecto original.

## Reportar un problema

Antes de abrir un reporte, comprueba que no exista ya uno similar. Incluye:

- Versión de Windows y arquitectura (por ejemplo, Windows 11 de 64 bits).
- Cómo ejecutas el programa: con Arduino, solo con `Configurar.cmd` o mediante
  `AudioDeejSwitch.exe --settings`.
- Pasos concretos para reproducir el problema.
- Qué esperabas que ocurriera y qué ocurrió realmente.
- Mensajes de error o capturas, si ayudan a entender el caso.

No publiques identificadores de dispositivos, rutas personales ni tu
`switcher.yaml` completo. Si necesitas compartir configuración, usa
`switcher.example.yaml` y sustituye los datos privados.

## Proponer una mejora

Abre una propuesta explicando el problema que resuelve y el comportamiento que
te gustaría obtener. Para cambios grandes, es mejor acordar el enfoque antes de
escribir código. Las mejoras deben mantener el objetivo del proyecto: una
experiencia sencilla para el usuario y compatibilidad con el Arduino y
`config.yaml` de deej.

## Enviar cambios

1. Crea una rama a partir de `main`.
2. Mantén el cambio centrado en un único tema.
3. Comprueba el formato y las pruebas antes de enviarlo:

   ```powershell
   gofmt -w .
   go test ./...
   go vet ./...
   .\build.ps1
   ```

4. Explica en la descripción qué cambia, por qué y cómo lo has probado.
5. Si el cambio afecta a la interfaz, adjunta una captura o describe el resultado.

No es necesario tener un Arduino conectado para la mayoría de las pruebas. Las
pruebas nativas enumeran dispositivos de audio y verifican el registro de atajos,
pero no cambian la salida ni envían comandos al Arduino. La comprobación audible
del cambio de salida y el funcionamiento de los potenciómetros requieren hardware
real.

## Entorno de desarrollo

- Windows 10/11 de 64 bits.
- Go 1.22 o posterior.
- PowerShell para `build.ps1`.

El proyecto se compila sin CGO y genera un único ejecutable para Windows. Las
herramientas auxiliares, como `github.com/akavel/rsrc`, se descargan en la versión
fijada por el script de compilación.

### Notificaciones

Los avisos de Windows usan `Shell_NotifyIcon` y no dependen de scripts ni de
PowerShell. Para comprobar el ciclo de vida del notificador nativo:

```powershell
$env:AUDIODEEJSWITCH_TEST_NOTIFICATIONS = '1'
go test ./pkg/deej -run '^TestWindowsNotifierLifecycle$' -count=1
Remove-Item Env:\AUDIODEEJSWITCH_TEST_NOTIFICATIONS
```

La prueba comprueba que Windows acepta la solicitud de aviso; no garantiza que
sea visible con los ajustes de notificaciones de tu escritorio.

## Estructura del proyecto

- `internal/audioswitch`: ventana de configuración, captura de atajos, cambio de
  salida y preferencias de la interfaz.
- `pkg/deej`: núcleo heredado de deej, incluido el control de volúmenes y la
  integración con la bandeja del sistema.
- `packaging`: manifiesto de Windows y generación del icono.
- `docs`: documentación adicional y esta guía.
- `arduino`: sketch de referencia para el hardware.

## Estilo de código

- Ejecuta `gofmt` sobre los archivos Go.
- Sigue el estilo y las convenciones del código cercano.
- Evita dependencias nuevas si la biblioteca estándar o las dependencias actuales
  resuelven el problema.
- Mantén los mensajes visibles para el usuario en español y coherentes con la
  interfaz existente.
- Añade o actualiza pruebas cuando cambies lógica de configuración, atajos o
  notificaciones.

## Licencia

Al contribuir aceptas que tus cambios se distribuyan bajo la licencia MIT del
proyecto. Consulta [`LICENSE`](../LICENSE).
