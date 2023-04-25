# nux

A Go implementation of the [Uxn](https://100r.co/site/uxn.html) virtual
machine, by [nf](https://nf.wh3rd.net/).

## Features

- Full support for Varvara's System, Console, Screen,
  Controller, Mouse, File, and Datetime devices.
- Live-reloading and rebuilding of uxntal source (`-dev`).
- An interactive debugger (`-debug`).
- Runs on macOS, Linux, and Windows (mostly tested/developed on macOS).

## Todo

- Implement the Audio device.

## Known issues

- The File device is not well-tested, and likely has bugs.
- The button events of the Mouse device somehow misfire.
- Included source files are not watched by the `-dev` feature.
- The GUI doesn't always shut down when exiting the debugger.
