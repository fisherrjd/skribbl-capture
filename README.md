# Skribbl Audio Capture

A cross-platform audio capture tool built in Go that can record from multiple audio devices simultaneously. Each device saves to its own separate WAV file, making it ideal for capturing both microphone input and system audio (via BlackHole) at the same time.

## Features

- Record from multiple audio devices simultaneously
- Each device outputs to its own WAV file
- CLI mode for quick terminal usage
- Web UI mode with a browser-based interface
- Cross-platform support (macOS, Windows, Linux) via [malgo](https://github.com/gen2brain/malgo)

## Requirements

- Go 1.25+
- For system audio capture on macOS: [BlackHole](https://existential.audio/blackhole/) (virtual audio driver)

## Installation

```bash
git clone https://github.com/yourusername/skribbl-capture.git
cd skribbl-capture
go mod tidy
```

## Usage

### CLI Mode

Run the tool directly in your terminal:

```bash
go run .
```

You'll see a list of available capture devices:

```
=== Available Capture Devices ===
[0] AirPods
[1] Microphone
[2] BlackHole 2ch
[3] MacBook Air Microphone

Enter device number(s) to capture from (comma-separated for multiple, e.g., 1,2):
```

Select one or more devices by entering their numbers separated by commas. Press Enter to stop recording. Each device saves to its own WAV file named after the device (e.g., `blackhole_2ch.wav`).

### Web Mode

Launch a browser-based interface:

```bash
go run . web
```

Then open http://localhost:8080 in your browser. The web UI lets you:

- Select devices with checkboxes
- Start/stop recording with buttons
- View and download past recordings

Recordings are saved to the `recordings/` directory with timestamps.

## Capturing System Audio on macOS

To capture system audio (e.g., game audio from Skribbl.io), you need to route it through BlackHole:

1. Install [BlackHole 2ch](https://existential.audio/blackhole/)
2. Open **Audio MIDI Setup** (in `/Applications/Utilities/`)
3. Click the **+** button and select **Create Multi-Output Device**
4. Check both your speakers/headphones (as primary) and **BlackHole 2ch**
5. Go to **System Settings > Sound > Output** and select the Multi-Output Device

Now system audio will be sent to both your speakers and BlackHole. Select **BlackHole 2ch** as a capture device in skribbl-capture to record system audio.

> **Note:** Volume controls are unavailable when using a Multi-Output Device. Use per-app volume controls or adjust levels in Audio MIDI Setup.

## Building

Build executables for distribution:

```bash
./build.sh
```

Binaries are output to the `dist/` directory. No Go installation is needed on the target machine - just copy the executable and run it.

> **Note:** Cross-compiling to Windows from macOS requires `mingw-w64` (`brew install mingw-w64`) due to CGo dependencies.

## Audio Format

All recordings are saved as WAV files with the following settings:

| Setting         | Value                         |
|-----------------|-------------------------------|
| Format          | PCM (uncompressed)            |
| Sample Rate     | 44,100 Hz (CD quality)        |
| Channels        | 1 (mono)                      |
| Bit Depth       | 16-bit signed integer         |

## Project Structure

```
skribbl-capture/
  main.go       - CLI mode, WAV header writing, entry point
  web.go        - Web server, API handlers
  index.html    - Web UI frontend
  build.sh      - Cross-platform build script
```
