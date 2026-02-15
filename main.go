package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gen2brain/malgo"
)

// writeWAVHeader writes the WAV file header
// sampleRate: samples per second (e.g., 44100)
// channels: number of audio channels (1 = mono, 2 = stereo)
// bitsPerSample: bits per sample (16 for our format)
// dataSize: total size of audio data in bytes (0 initially, we'll update later)
func writeWAVHeader(file *os.File, sampleRate, channels, bitsPerSample, dataSize uint32) error {
	// WAV file structure:
	// "RIFF" chunk descriptor
	file.WriteString("RIFF")
	binary.Write(file, binary.LittleEndian, uint32(36+dataSize)) // File size - 8
	file.WriteString("WAVE")

	// "fmt " sub-chunk (format)
	file.WriteString("fmt ")
	binary.Write(file, binary.LittleEndian, uint32(16))                          // Subchunk size
	binary.Write(file, binary.LittleEndian, uint16(1))                           // Audio format (1 = PCM)
	binary.Write(file, binary.LittleEndian, uint16(channels))                    // Number of channels
	binary.Write(file, binary.LittleEndian, sampleRate)                          // Sample rate
	binary.Write(file, binary.LittleEndian, sampleRate*channels*bitsPerSample/8) // Byte rate
	binary.Write(file, binary.LittleEndian, uint16(channels*bitsPerSample/8))    // Block align
	binary.Write(file, binary.LittleEndian, uint16(bitsPerSample))               // Bits per sample

	// "data" sub-chunk
	file.WriteString("data")
	binary.Write(file, binary.LittleEndian, dataSize) // Data size

	return nil
}

// captureDevice holds all the state for a single audio capture device
type captureDevice struct {
	name             string
	file             *os.File
	device           *malgo.Device
	totalBytesWritten uint32
}

func main() {
	fmt.Println("Skribbl Audio Capture")

	// Step 1: Initialize the malgo context
	// This sets up the audio backend for your platform (CoreAudio on Mac, WASAPI on Windows)
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		fmt.Printf("Failed to initialize audio context: %v\n", err)
		return
	}
	defer ctx.Uninit()

	fmt.Println("Audio context initialized successfully!")

	// Step 2: List all available audio devices
	// We'll look for "capture" devices (microphones, system audio inputs)
	fmt.Println("\n=== Available Capture Devices ===")

	// Get list of all playback and capture devices
	infos, err := ctx.Devices(malgo.Capture)
	if err != nil {
		fmt.Printf("Failed to get devices: %v\n", err)
		return
	}

	// Loop through and print each device
	for i, info := range infos {
		fmt.Printf("[%d] %s\n", i, info.Name())
	}

	// Step 3: Ask user to select devices (comma-separated for multiple)
	fmt.Println("\nEnter device number(s) to capture from (comma-separated for multiple, e.g., 1,2):")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Failed to read input: %v\n", err)
		return
	}

	// Clean up the input (removes the newline character and any spaces)
	input = strings.TrimSpace(input)

	// Parse each comma-separated value into device indices
	parts := strings.Split(input, ",")
	selectedIndices := []int{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		deviceIndex, err := strconv.Atoi(part)
		if err != nil {
			fmt.Printf("That's not a valid number: %s\n", part)
			return
		}
		if deviceIndex < 0 || deviceIndex >= len(infos) {
			fmt.Printf("Invalid device! Please choose 0-%d\n", len(infos)-1)
			return
		}
		selectedIndices = append(selectedIndices, deviceIndex)
	}

	if len(selectedIndices) == 0 {
		fmt.Println("No devices selected!")
		return
	}

	// Step 4: Set up capture for each selected device
	captures := []*captureDevice{}

	for _, idx := range selectedIndices {
		deviceInfo := infos[idx]
		deviceName := deviceInfo.Name()
		fmt.Printf("\nSetting up: %s\n", deviceName)

		// Create a safe filename from the device name (replace spaces with underscores)
		safeFilename := strings.ReplaceAll(strings.ToLower(deviceName), " ", "_") + ".wav"

		// Configure the audio capture settings
		deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
		deviceConfig.Capture.Format = malgo.FormatS16 // 16-bit audio samples
		deviceConfig.Capture.Channels = 1             // Mono (single channel audio)
		deviceConfig.SampleRate = 44100                // 44,100 samples per second (CD quality)
		deviceConfig.Capture.DeviceID = deviceInfo.ID.Pointer()

		// Create a file for this device
		outputFile, err := os.Create(safeFilename)
		if err != nil {
			fmt.Printf("Failed to create output file for %s: %v\n", deviceName, err)
			return
		}

		// Write the WAV header (with dataSize = 0 for now, we'll update it later)
		err = writeWAVHeader(outputFile, deviceConfig.SampleRate, uint32(deviceConfig.Capture.Channels), 16, 0)
		if err != nil {
			fmt.Printf("Failed to write WAV header for %s: %v\n", deviceName, err)
			return
		}

		// Create a captureDevice to track this device's state
		cap := &captureDevice{
			name: deviceName,
			file: outputFile,
		}
		captures = append(captures, cap)

		// Define the callback for this device
		// Each device gets its own callback that writes to its own file
		onRecvFrames := func(pSample2, pSample []byte, framecount uint32) {
			n, err := cap.file.Write(pSample)
			if err != nil {
				fmt.Printf("Error writing audio data for %s: %v\n", cap.name, err)
			}
			cap.totalBytesWritten += uint32(n)
		}

		// Initialize the device with our config and callback
		device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
			Data: onRecvFrames,
		})
		if err != nil {
			fmt.Printf("Failed to initialize device %s: %v\n", deviceName, err)
			return
		}
		cap.device = device

		fmt.Printf("✓ %s → %s\n", deviceName, safeFilename)
	}

	// Step 5: Start all devices
	for _, cap := range captures {
		err := cap.device.Start()
		if err != nil {
			fmt.Printf("Failed to start device %s: %v\n", cap.name, err)
			return
		}
		fmt.Printf("🎙️  Started recording: %s\n", cap.name)
	}

	fmt.Println("\nPress Enter to stop recording...")
	reader.ReadString('\n')

	fmt.Println("\nRecording stopped!")

	// Step 6: Clean up - stop devices, update WAV headers, close files
	for _, cap := range captures {
		cap.device.Uninit()

		// Go back to the beginning of the file and rewrite the header with correct size
		cap.file.Seek(0, 0)
		writeWAVHeader(cap.file, 44100, 1, 16, cap.totalBytesWritten)
		cap.file.Close()

		fmt.Printf("✓ Saved %s (%d bytes of audio)\n", cap.name, cap.totalBytesWritten)
	}

	fmt.Println("✓ All recordings saved!")
}
