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
   (por ejemplo `Ctrl+Alt+F8`), pulsa **Añadir atajo** y después **Guardar atajos**.
   También puedes asignar un atajo a **Siguiente salida disponible (alternar)**.

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
  segundo sin reiniciar deej. Un YAML de atajos inválido conserva la configuración
  activa anterior. Los atajos ocupados o reservados se notifican y no se registran.

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

El script genera el recurso de Windows con el manifiesto necesario para la ventana
y compila `AudioDeejSwitch.exe` sin consola. Descarga la herramienta de recursos
`github.com/akavel/rsrc` en su versión fijada `v0.10.2`.

Las pruebas nativas enumeran dispositivos, comprueban la disponibilidad de la
interfaz de cambio de salida y verifican registro, recarga y liberación de atajos
reales de Windows. **No cambian la salida ni envían comandos al Arduino.**
Las pruebas de configuración comprueban validación, duplicados, respaldo y
protección frente a ediciones externas. El cambio audible entre dispositivos y
el funcionamiento de los potenciómetros requieren una comprobación con tu equipo.

### Créditos

Base: [omriharel/deej](https://github.com/omriharel/deej), licencia MIT conservada
en `LICENSE`. La documentación original sigue debajo. Las funciones de selección
y atajos están inspiradas en [Audio Switcher](https://github.com/xenolightning/AudioSwitcher_v1);
esta implementación las añade en Go usando las APIs de Windows y no requiere
instalar Audio Switcher ni incorpora su código C#.

---

# deej — documentación original

deej is an **open-source hardware volume mixer** for Windows and Linux PCs. It lets you use real-life sliders (like a DJ!) to **seamlessly control the volumes of different apps** (such as your music player, the game you're playing and your voice chat session) without having to stop what you're doing.

**Join the [deej Discord server](https://discord.gg/nf88NJu) if you need help or have any questions!**

[![Discord](https://img.shields.io/discord/702940502038937667?logo=discord)](https://discord.gg/nf88NJu)

> **_New:_** [work-in-progress deej FAQ](./docs/faq/faq.md)!

deej consists of a [lightweight desktop client](#features) written in Go, and an Arduino-based hardware setup that's simple and cheap to build. [**Check out some versions built by members of our community!**](./community.md)

**[Download the latest release](https://github.com/omriharel/deej/releases/latest) | [Video demonstration](https://youtu.be/VoByJ4USMr8) | [Build video by Tech Always](https://youtu.be/x2yXbFiiAeI)**

![deej](assets/build-3d-annotated.png)

> _**Psst!** [No 3D printer? No problem!](./assets/build-shoebox.jpg)_ You can build deej on some cardboard, a shoebox or even a breadboard :)

## Table of contents

- [Features](#features)
- [How it works](#how-it-works)
  - [Hardware](#hardware)
    - [Schematic](#schematic)
  - [Software](#software)
- [Slider mapping (configuration)](#slider-mapping-configuration)
- [Build your own!](#build-your-own)
  - [FAQ](#faq)
  - [Build video](#build-video)
  - [Bill of Materials](#bill-of-materials)
  - [Thingiverse collection](#thingiverse-collection)
  - [Build procedure](#build-procedure)
- [How to run](#how-to-run)
  - [Requirements](#requirements)
  - [Download and installation](#download-and-installation)
  - [Building from source](#building-from-source)
- [Community](#community)
- [License](#license)

## Features

deej is written in Go and [distributed](https://github.com/omriharel/deej/releases/latest) as a portable (no installer needed) executable.

- Bind apps to different sliders
  - Bind multiple apps per slider (i.e. one slider for all your games)
  - Bind the master channel
  - Bind "system sounds" (on Windows)
  - Bind specific audio devices by name (on Windows)
  - Bind currently active app (on Windows)
  - Bind all other unassigned apps
- Control your microphone's input level
- Lightweight desktop client, consuming around 10MB of memory
- Runs from your system tray
- Helpful notifications to let you know if something isn't working

> **Looking for the older Python version?** It's no longer maintained, but you can always find it in the [`legacy-python` branch](https://github.com/omriharel/deej/tree/legacy-python).

## How it works

### Hardware

- The sliders are connected to 5 (or as many as you like) analog pins on an Arduino Nano/Uno board. They're powered from the board's 5V output (see schematic)
- The board connects via a USB cable to the PC

#### Schematic

![Hardware schematic](assets/schematic.png)

### Software

- The code running on the Arduino board is a [C program](./arduino/deej-5-sliders-vanilla/deej-5-sliders-vanilla.ino) constantly writing current slider values over its serial interface
- The PC runs a lightweight [Go client](./pkg/deej/cmd/main.go) in the background. This client reads the serial stream and adjusts app volumes according to the given configuration file

## Slider mapping (configuration)

deej uses a simple YAML-formatted configuration file named [`config.yaml`](./config.yaml), placed alongside the deej executable.

The config file determines which applications (and devices) are mapped to which sliders, and which parameters to use for the connection to the Arduino board, as well as other user preferences.

**This file auto-reloads when its contents are changed, so you can change application mappings on-the-fly without restarting deej.**

It looks like this:

```yaml
slider_mapping:
  0: master
  1: chrome.exe
  2: spotify.exe
  3:
    - pathofexile_x64.exe
    - rocketleague.exe
  4: discord.exe

# set this to true if you want the controls inverted (i.e. top is 0%, bottom is 100%)
invert_sliders: false

# settings for connecting to the arduino board
com_port: COM4
baud_rate: 9600

# adjust the amount of signal noise reduction depending on your hardware quality
# supported values are "low" (excellent hardware), "default" (regular hardware) or "high" (bad, noisy hardware)
noise_reduction: default
```

- `master` is a special option to control the master volume of the system _(uses the default playback device)_
- `mic` is a special option to control your microphone's input level _(uses the default recording device)_
- `deej.unmapped` is a special option to control all apps that aren't bound to any slider ("everything else")
- On Windows, `deej.current` is a special option to control whichever app is currently in focus
- On Windows, you can specify a device's full name, i.e. `Speakers (Realtek High Definition Audio)`, to bind that device's level to a slider. This doesn't conflict with the default `master` and `mic` options, and works for both input and output devices.
  - Be sure to use the full device name, as seen in the menu that comes up when left-clicking the speaker icon in the tray menu
- `system` is a special option on Windows to control the "System sounds" volume in the Windows mixer
- All names are case-**in**sensitive, meaning both `chrome.exe` and `CHROME.exe` will work
- You can create groups of process names (using a list) to either:
    - control more than one app with a single slider
    - choose whichever process in the group that's currently running (i.e. to have one slider control any game you're playing)

## Build your own!

Building deej is very simple. You only need a few relatively cheap parts - it's an excellent starter project (and my first Arduino project, personally). Remember that if you need any help or have a question that's not answered here, you can always [join the deej Discord server](https://discord.gg/nf88NJu).

Build deej for yourself, or as an awesome gift for your gaming buddies!

### FAQ

I've started a highly focused effort of writing a proper FAQ page for deej, covering many basic and advanced topics.

It is still _very much a work-in-progress_, but I'm happy to [share it in its current state](./docs/faq/faq.md) in hopes that it at least covers some questions you might have.

FAQ feedback in our [community Discord](https://discord.gg/nf88NJu) is strongly encouraged :)

### Build video

In case you prefer watching to reading, Charles from the [**Tech Always**](https://www.youtube.com/c/TechAlways) YouTube channel has made [**a fantastic video**](https://youtu.be/x2yXbFiiAeI) that covers the basics of building deej for yourself, including parts, costs, assembly and software. I highly recommend checking it out!

### Bill of Materials

- An Arduino Nano, Pro Micro or Uno board
  - I officially recommend using a Nano or a Pro Micro for their smaller form-factor, friendlier USB connectors and more analog pins. Plus they're cheaper
  - You can also use any other development board that has a Serial over USB interface
- A few slider potentiometers, up to your number of free analog pins (the cheaper ones cost around 1-2 USD each, and come with a standard 10K Ohm variable resistor. These _should_ work just fine for this project)
  - **Important:** make sure to get **linear** sliders, not logarithmic ones! Check the product description
  - You can also use circular knobs if you like
- Some wires
- Any kind of box to hold everything together. **You don't need a 3D printer for this project!** It works fantastically with just a piece of cardboard or a shoebox. That being said, if you do have one, read on...

### Thingiverse collection

With many different 3D-printed designs being added to our [community showcase](./community.md), it felt right to gather all of them in a Thingiverse collection for you to browse. If you have access to a 3D printer, feel free to use one of the designs in your build.

**[Visit our community-created design collection on Thingiverse!](https://thingiverse.com/omriharel/collections/deej)**

> You can also [submit your own](https://discord.gg/nf88NJu) design to be added to the collection. Regardless, if you do upload your design to Thingiverse, _please add a `deej` tag to it so that others can find it more easily_.


### Build procedure

- Connect everything according to the [schematic](#schematic)
- Test with a multimeter to be sure your sliders are hooked up correctly
- Flash the Arduino chip with the sketch in [`arduino\deej-5-sliders-vanilla`](./arduino/deej-5-sliders-vanilla/deej-5-sliders-vanilla.ino)
  - _Important:_ If you have more or less than 5 sliders, you must edit the sketch to match what you have
- After flashing, check the serial monitor. You should see a constant stream of values separated by a pipe (`|`) character, e.g. `0|240|1023|0|483`
  - When you move a slider, its corresponding value should move between 0 and 1023
- Congratulations, you're now ready to run the deej executable!

## How to run

### Requirements

#### Windows

- Windows. That's it

#### Linux

- Install `libgtk-3-dev`, `libappindicator3-dev` and `libwebkit2gtk-4.0-dev` for system tray support. Pre-built Linux binaries aren't currently released, so you'll need to [build from source](#building-from-source). If there's demand for pre-built binaries, please [let me know](https://discord.gg/nf88NJu)!

### Download and installation

- Head over to the [releases page](https://github.com/omriharel/deej/releases) and download the [latest version](https://github.com/omriharel/deej/releases/latest)'s executable and configuration file (`deej.exe` and `config.yaml`)
- Place them in the same directory anywhere on your machine
- (Optional, on Windows) Create a shortcut to `deej.exe` and copy it to `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup` to have deej run on boot

### Building from source

If you'd rather not download a compiled executable, or want to extend deej or modify it to your needs, feel free to clone the repository and build it yourself. All you need is a Go 1.14 (or above) environment on your machine. If you go this route, make sure to check out the [developer scripts](./pkg/deej/scripts).

Like other Go packages, you can also use the `go get` tool: `go get -u github.com/omriharel/deej`. Please note that the package code now resides in the `pkg/deej` directory, and needs to be imported from there if used inside another project.

If you need any help with this, please [join our Discord server](https://discord.gg/nf88NJu).

## Community

[![Discord](https://img.shields.io/discord/702940502038937667?logo=discord)](https://discord.gg/nf88NJu)

deej is a relatively new project, but a vibrant and awesome community is rapidly growing around it. Come hang out with us in the [deej Discord server](https://discord.gg/nf88NJu), or check out a whole bunch of cool and creative builds made by our members in the [community showcase](./community.md).

The server is also a great place to ask questions, suggest features or report bugs (but of course, feel free to use GitHub if you prefer).

### Donations

If you love deej and want to show your support for this project, you can do so using the link below. Please don't feel obligated to donate - building the project and telling your friends about it goes a very long way! Thank you very much.

[![ko-fi](https://www.ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/omriharel)

### Contributing

Please see [`docs/CONTRIBUTING.md`](./docs/CONTRIBUTING.md).

## License

deej is released under the [MIT license](./LICENSE).
