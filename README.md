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

- The button events of the Mouse device somehow misfire.
- The GUI doesn't always shut down when exiting the debugger.
