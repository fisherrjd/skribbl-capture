package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

var (
	// Global state for recording
	isRecording     bool
	recordingMutex  sync.Mutex
	activeCaptures  []*captureDevice
	malgoContext    *malgo.AllocatedContext
	outputDirectory = "recordings"
)

// DeviceInfo represents an audio device for the API
type DeviceInfo struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Type  string `json:"type"` // "capture" or "loopback"
}

// RecordingStatus represents the current recording state
type RecordingStatus struct {
	IsRecording bool     `json:"isRecording"`
	Devices     []string `json:"devices"`
}

// StartRecordingRequest is the request body for starting a recording
type StartRecordingRequest struct {
	DeviceIndices []int `json:"deviceIndices"`
}

func initWebServer() error {
	// Create recordings directory if it doesn't exist
	if err := os.MkdirAll(outputDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create recordings directory: %v", err)
	}

	// Initialize malgo context
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize audio context: %v", err)
	}
	malgoContext = ctx

	return nil
}

// Handler: GET /api/devices - List all available capture devices
func handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices := []DeviceInfo{}

	// List capture devices (microphones, virtual inputs)
	captureInfos, err := malgoContext.Devices(malgo.Capture)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get capture devices: %v", err), http.StatusInternalServerError)
		return
	}
	for i, info := range captureInfos {
		devices = append(devices, DeviceInfo{
			Index: i,
			Name:  info.Name(),
			Type:  "capture",
		})
	}

	// On Windows, also list playback devices as loopback sources
	if runtime.GOOS == "windows" {
		playbackInfos, err := malgoContext.Devices(malgo.Playback)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get playback devices: %v", err), http.StatusInternalServerError)
			return
		}
		for i, info := range playbackInfos {
			devices = append(devices, DeviceInfo{
				Index: len(captureInfos) + i,
				Name:  info.Name(),
				Type:  "loopback",
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// Handler: GET /api/status - Get current recording status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	recordingMutex.Lock()
	defer recordingMutex.Unlock()

	deviceNames := []string{}
	for _, cap := range activeCaptures {
		deviceNames = append(deviceNames, cap.name)
	}

	status := RecordingStatus{
		IsRecording: isRecording,
		Devices:     deviceNames,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Handler: POST /api/start - Start recording
func handleStartRecording(w http.ResponseWriter, r *http.Request) {
	recordingMutex.Lock()
	defer recordingMutex.Unlock()

	if isRecording {
		http.Error(w, "Already recording", http.StatusBadRequest)
		return
	}

	var req StartRecordingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.DeviceIndices) == 0 {
		http.Error(w, "No devices selected", http.StatusBadRequest)
		return
	}

	// Build unified device list (capture + loopback on Windows)
	allDevices := []selectableDevice{}

	captureInfos, err := malgoContext.Devices(malgo.Capture)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get capture devices: %v", err), http.StatusInternalServerError)
		return
	}
	for _, info := range captureInfos {
		allDevices = append(allDevices, selectableDevice{info: info, isLoopback: false})
	}

	if runtime.GOOS == "windows" {
		playbackInfos, err := malgoContext.Devices(malgo.Playback)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get playback devices: %v", err), http.StatusInternalServerError)
			return
		}
		for _, info := range playbackInfos {
			allDevices = append(allDevices, selectableDevice{info: info, isLoopback: true})
		}
	}

	// Set up capture for each selected device
	captures := []*captureDevice{}
	timestamp := time.Now().Format("2006-01-02_15-04-05")

	for _, idx := range req.DeviceIndices {
		if idx < 0 || idx >= len(allDevices) {
			http.Error(w, fmt.Sprintf("Invalid device index: %d", idx), http.StatusBadRequest)
			return
		}

		selected := allDevices[idx]
		deviceInfo := selected.info
		deviceName := deviceInfo.Name()

		// Create filename with timestamp
		safeFilename := fmt.Sprintf("%s_%s.wav", timestamp, sanitizeFilename(deviceName))
		fullPath := filepath.Join(outputDirectory, safeFilename)

		// Configure the audio capture settings
		// Use Loopback mode for playback devices on Windows, Capture for regular mics
		deviceType := malgo.Capture
		if selected.isLoopback {
			deviceType = malgo.Loopback
		}
		deviceConfig := malgo.DefaultDeviceConfig(deviceType)
		deviceConfig.Capture.Format = malgo.FormatS16
		deviceConfig.Capture.Channels = 1
		deviceConfig.SampleRate = 44100
		deviceConfig.Capture.DeviceID = deviceInfo.ID.Pointer()

		// Create output file
		outputFile, err := os.Create(fullPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create file: %v", err), http.StatusInternalServerError)
			return
		}

		// Write WAV header
		if err := writeWAVHeader(outputFile, deviceConfig.SampleRate, uint32(deviceConfig.Capture.Channels), 16, 0); err != nil {
			outputFile.Close()
			http.Error(w, fmt.Sprintf("Failed to write WAV header: %v", err), http.StatusInternalServerError)
			return
		}

		// Create capture device
		cap := &captureDevice{
			name:     deviceName,
			file:     outputFile,
			filename: fullPath,
		}
		captures = append(captures, cap)

		// Define callback
		onRecvFrames := func(pSample2, pSample []byte, framecount uint32) {
			n, _ := cap.file.Write(pSample)
			cap.totalBytesWritten += uint32(n)
		}

		// Initialize device
		device, err := malgo.InitDevice(malgoContext.Context, deviceConfig, malgo.DeviceCallbacks{
			Data: onRecvFrames,
		})
		if err != nil {
			outputFile.Close()
			http.Error(w, fmt.Sprintf("Failed to initialize device: %v", err), http.StatusInternalServerError)
			return
		}
		cap.device = device

		// Start device
		if err := device.Start(); err != nil {
			device.Uninit()
			outputFile.Close()
			http.Error(w, fmt.Sprintf("Failed to start device: %v", err), http.StatusInternalServerError)
			return
		}
	}

	activeCaptures = captures
	isRecording = true

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "recording started"})
}

// Handler: POST /api/stop - Stop recording
func handleStopRecording(w http.ResponseWriter, r *http.Request) {
	recordingMutex.Lock()
	defer recordingMutex.Unlock()

	if !isRecording {
		http.Error(w, "Not currently recording", http.StatusBadRequest)
		return
	}

	// Stop all devices and clean up
	for _, cap := range activeCaptures {
		cap.device.Uninit()

		// Update WAV header with correct size
		cap.file.Seek(0, 0)
		writeWAVHeader(cap.file, 44100, 1, 16, cap.totalBytesWritten)
		cap.file.Close()
	}

	activeCaptures = []*captureDevice{}
	isRecording = false

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "recording stopped"})
}

// Handler: GET /api/recordings - List all recordings
func handleListRecordings(w http.ResponseWriter, r *http.Request) {
	files, err := filepath.Glob(filepath.Join(outputDirectory, "*.wav"))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list recordings: %v", err), http.StatusInternalServerError)
		return
	}

	recordings := []map[string]interface{}{}
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		recordings = append(recordings, map[string]interface{}{
			"name": filepath.Base(file),
			"size": info.Size(),
			"time": info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recordings)
}

// Handler: GET /recordings/{filename} - Download a recording
func handleDownloadRecording(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path[len("/recordings/"):]
	fullPath := filepath.Join(outputDirectory, filepath.Base(filename))

	// Security: prevent directory traversal
	if !filepath.HasPrefix(fullPath, outputDirectory) {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	http.ServeFile(w, r, fullPath)
}

func sanitizeFilename(name string) string {
	// Replace spaces and special characters with underscores
	result := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			result += string(r)
		} else {
			result += "_"
		}
	}
	return result
}
