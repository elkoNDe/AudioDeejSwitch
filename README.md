# AudioDeejSwitch

Versión para **Windows 10/11 de 64 bits** basada en deej: conserva el Arduino,
el protocolo serie y `config.yaml`, y añade selección de salida y atajos globales.

## Inicio rápido

1. Cierra tu deej anterior para liberar el puerto COM.
2. Copia **tu `config.yaml` actual** a esta carpeta, junto a `AudioDeejSwitch.exe`.
   El archivo incluido es el ejemplo original; comprueba el puerto COM antes de usarlo.
3. Ejecuta `AudioDeejSwitch.exe`. El mezclador queda en la bandeja del sistema.
4. Clic derecho en su icono → **Configuración**.
5. Selecciona una salida, pulsa **Capturar**, pulsa la combinación que quieras
   (por ejemplo `Ctrl+Alt+F8`) y pulsa **Añadir atajo**. El atajo se guarda
   automáticamente. También puedes asignar un atajo a
   **Siguiente salida disponible (alternar)**.

Para abrir la configuración sin conectar con el Arduino, ejecuta **`Configurar.cmd`**
o `AudioDeejSwitch.exe --settings`. Este modo permite preparar la configuración,
pero los atajos necesitan la aplicación principal ejecutándose en la bandeja.

### Qué incluye

- Control de volúmenes original de deej: aplicaciones, grupos, master, mic,
  sonidos del sistema, aplicación en primer plano y aplicaciones sin asignar.
- Ventana nativa pequeña para seleccionar salida y configurar atajos globales.
- Atajos directos a un dispositivo o para alternar entre todas las salidas activas,
  ordenadas por nombre. No necesitan que la ventana tenga el foco ni siga abierta.
- Cambio de salida predeterminada de consola y multimedia, sin avisos al cambiar
  correctamente. No modifica la salida de comunicaciones ni la entrada de micrófono.
- Configuración con apariencia oscura e icono de deej con degradado rojo a negro
  en el ejecutable y la bandeja del sistema.
- Editor de `config.yaml` integrado: conserva el texto y los comentarios,
  valida antes de guardar y crea `config.yaml.bak` con la versión anterior.
- Detección de ediciones externas: si el archivo cambió después de abrir la
  ventana, se evita sobrescribirlo. En el editor de deej puedes recargarlo;
  para recargar los atajos, cierra y vuelve a abrir la ventana.
- Configuración de atajos en `switcher.yaml`, recargada en aproximadamente un
  segundo sin reiniciar deej. Los cambios se guardan automáticamente al añadir
  o eliminar un atajo. Un YAML de atajos inválido conserva la configuración
  activa anterior. Los atajos ocupados o reservados se notifican y no se registran.
  Tienes una plantilla sin datos personales en `switcher.example.yaml`.

Se admiten combinaciones de Ctrl, Alt, Shift o Win con cualquier tecla física del
teclado: letras, números, F1-F24, teclas OEM como `Oem5`, flechas, teclado numérico,
multimedia, etc. La captura guarda la tecla física, no el carácter: por ejemplo,
`Shift+Oem1` puede escribir `:` según la distribución del teclado. F12 está
reservada por Windows. Los IDs de los dispositivos se guardan para distinguir
salidas con nombres iguales. Si un dispositivo está desconectado, su atajo muestra
un aviso; vuelve a funcionar cuando regresa con el mismo ID.

Las aplicaciones configuradas para una salida concreta pueden mantenerla al
cambiar la predeterminada de Windows. Algunas necesitan reiniciar la reproducción.
Esta versión cambia la salida de Windows; no asigna rutas de audio por aplicación.

### Archivos y arranque

Mantén el ejecutable, `config.yaml` y `switcher.yaml` en una carpeta con permisos
de escritura. Los archivos se buscan junto al ejecutable, incluido al arrancar
desde un acceso directo. Puedes colocar un acceso directo al ejecutable en
`shell:startup` para iniciarlo con Windows.

### Compilación y comprobaciones

Necesitas Go 1.22 o posterior. Desde esta carpeta, en PowerShell:

```powershell
.\build.ps1
go test ./...
go vet ./...
```

El script verifica la integridad de las dependencias, genera el recurso de Windows
con el manifiesto necesario para la ventana y compila `AudioDeejSwitch.exe` sin
consola. Descarga la herramienta de recursos `github.com/akavel/rsrc` en su versión
fijada `v0.10.2` y rechaza los antiguos backends de notificaciones basados en scripts.

Para regenerar también todo el código compilado, usa `.\build.ps1 -Clean`.

En Windows, los avisos usan directamente `Shell_NotifyIcon`: no generan scripts
ni ejecutan PowerShell. Mientras se muestra un aviso puede aparecer un icono
adicional de notificación, que se retira tras 15 segundos sin mensajes nuevos.
Windows puede ocultar los avisos según sus ajustes de notificaciones.

Las pruebas nativas enumeran dispositivos, comprueban la disponibilidad de la
interfaz de cambio de salida y verifican registro, recarga y liberación de atajos
reales de Windows. **No cambian la salida ni envían comandos al Arduino.**
Las pruebas de configuración comprueban validación, duplicados, respaldo y
protección frente a ediciones externas. El cambio audible entre dispositivos y
el funcionamiento de los potenciómetros requieren una comprobación con tu equipo.

Para comprobar también el envío de un aviso nativo de Windows:

```powershell
$env:AUDIODEEJSWITCH_TEST_NOTIFICATIONS = '1'
go test ./pkg/deej -run '^TestWindowsNotifierLifecycle$' -count=1
Remove-Item Env:\AUDIODEEJSWITCH_TEST_NOTIFICATIONS
```

La prueba solicita un aviso y retira el icono al terminar; comprueba que Windows
acepta la solicitud, no que el aviso sea visible con los ajustes de tu escritorio.
La investigación de la detección de Defender y sus resultados están en
[`docs/defender-review.md`](docs/defender-review.md).

### Contribuir

Lee [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md). Este proyecto es un fork
independiente: los cambios y las propuestas se gestionan en este repositorio,
no en el de deej original.

### Desarrollo asistido por IA

Parte del código y de la documentación se desarrolló con ayuda de herramientas de
IA. El código generado se revisa, compila y prueba antes de publicarse; las
decisiones de diseño y la publicación siguen siendo responsabilidad del
mantenedor. El historial del repositorio conserva las contribuciones originales
de deej.

### Créditos

Base: [omriharel/deej](https://github.com/omriharel/deej), con su documentación y
licencia MIT conservada en `LICENSE`. Consulta la documentación original en el
[repositorio de deej](https://github.com/omriharel/deej#readme). Las funciones de
selección y atajos están inspiradas en
[Audio Switcher](https://github.com/xenolightning/AudioSwitcher_v1); esta
implementación las añade en Go usando las APIs de Windows y no requiere instalar
Audio Switcher ni incorpora su código C#.

## Licencia

AudioDeejSwitch se distribuye bajo la [licencia MIT](LICENSE).
