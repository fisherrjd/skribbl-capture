package main

import (
	"fmt"
	"bufio"
	"os"
	"strconv"
	"strings"
	"encoding/binary"

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
	binary.Write(file, binary.LittleEndian, uint32(16))          // Subchunk size
	binary.Write(file, binary.LittleEndian, uint16(1))           // Audio format (1 = PCM)
	binary.Write(file, binary.LittleEndian, uint16(channels))    // Number of channels
	binary.Write(file, binary.LittleEndian, sampleRate)          // Sample rate
	binary.Write(file, binary.LittleEndian, sampleRate*channels*bitsPerSample/8) // Byte rate
	binary.Write(file, binary.LittleEndian, uint16(channels*bitsPerSample/8))    // Block align
	binary.Write(file, binary.LittleEndian, uint16(bitsPerSample)) // Bits per sample

	// "data" sub-chunk
	file.WriteString("data")
	binary.Write(file, binary.LittleEndian, dataSize) // Data size

	return nil
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

	// Step 3: Ask user to select a device
	fmt.Println("\nEnter device number:")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Failed to read input: %v\n", err)
		return
	}

	// Clean up the input (removes the newline character and any spaces)
	input = strings.TrimSpace(input)

	// Convert the text string to a number
	deviceIndex, err := strconv.Atoi(input)
	if err != nil {
		fmt.Printf("That's not a valid number: %v\n", err)
		return
	}

	// Make sure the number is within the valid range
	if deviceIndex < 0 || deviceIndex >= len(infos) {
		fmt.Printf("Invalid device! Please choose 0-%d\n", len(infos)-1)
		return
	}

	// Get the device from our array
	selectedDevice := infos[deviceIndex]
	fmt.Printf("✓ You selected: %s\n", selectedDevice.Name())

	// Step 4: Configure the audio capture settings
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16       // 16-bit audio samples
	deviceConfig.Capture.Channels = 1                    // Mono (single channgel audio))
	deviceConfig.SampleRate = 44100                      // 44,100 samples per second (CD quality)
	deviceConfig.Capture.DeviceID = selectedDevice.ID.Pointer()  // Tell it which device to use

	fmt.Printf("Configured for %d Hz, %d channels\n", deviceConfig.SampleRate, deviceConfig.Capture.Channels)

	// Step 5: Create a file to save the audio
	outputFile, err := os.Create("recording.wav")
	if err != nil {
		fmt.Printf("Failed to create output file: %v\n", err)
		return
	}
	defer outputFile.Close()  // Make sure we close the file when done

	// Write the WAV header (with dataSize = 0 for now, we'll update it later)
	err = writeWAVHeader(outputFile, deviceConfig.SampleRate, uint32(deviceConfig.Capture.Channels), 16, 0)
	if err != nil {
		fmt.Printf("Failed to write WAV header: %v\n", err)
		return
	}

	fmt.Println("✓ Created recording.wav with WAV header")

	// Track how much audio data we write
	var totalBytesWritten uint32 = 0

	// Step 6: Define the callback function (now we have access to outputFile!)
	onRecvFrames := func(pSample2, pSample []byte, framecount uint32) {
		// Write the raw audio data directly to the file
		n, err := outputFile.Write(pSample)
		if err != nil {
			fmt.Printf("Error writing audio data: %v\n", err)
		}
		totalBytesWritten += uint32(n)  // Keep track of total bytes
		fmt.Printf("📊 Wrote %d bytes to file (total: %d)\n", len(pSample), totalBytesWritten)
	}

	// Step 7: Initialize the device with our config and callback
	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onRecvFrames,  // Tell it to call our function when data arrives
	})
	if err != nil {
		fmt.Printf("Failed to initialize device: %v\n", err)
		return
	}
	defer device.Uninit()  // Make sure we clean up when done

	fmt.Println("✓ Device initialized!")

	// Step 8: Start capturing audio!
	err = device.Start()
	if err != nil {
		fmt.Printf("Failed to start device: %v\n", err)
		return
	}

	fmt.Println("🎙️  Recording... Press Enter to stop")
	reader.ReadString('\n')  // Wait for user to press Enter

	fmt.Println("Recording stopped!")

	// Step 9: Update the WAV header with the correct file size
	// Go back to the beginning of the file
	outputFile.Seek(0, 0)

	// Rewrite the header with the actual data size
	err = writeWAVHeader(outputFile, deviceConfig.SampleRate, uint32(deviceConfig.Capture.Channels), 16, totalBytesWritten)
	if err != nil {
		fmt.Printf("Failed to update WAV header: %v\n", err)
		return
	}

	fmt.Printf("✓ Updated header with %d bytes of audio data\n", totalBytesWritten)
	fmt.Println("✓ Recording saved to recording.wav")
}
