package logservice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxDeviceLogDecodeBytes = 16 << 20

var deviceLogXORKey = [8]byte{0x7e, 0xa3, 0x1f, 0x58, 0xc4, 0x9b, 0x36, 0xe2}

// decodeDeviceLogFiles decodes the 70mai rolling logfile format before parsing.
// Archive contents are replaced in place; direct uploads keep their original copy.
func decodeDeviceLogFiles(itemRoot string, files []LogFile) (int, error) {
	decodedCount := 0
	for i := range files {
		if !shouldTryDeviceLogDecode(files[i].RelativePath) {
			continue
		}
		path := filepath.Join(itemRoot, filepath.FromSlash(files[i].RelativePath))
		decoded, changed, err := decodeDeviceLogFile(path)
		if err != nil {
			return decodedCount, fmt.Errorf("decode device log %q: %w", files[i].RelativePath, err)
		}
		if !changed {
			continue
		}
		if strings.HasPrefix(files[i].RelativePath, "original/") {
			files[i].RelativePath = filepath.ToSlash(filepath.Join("decoded", filepath.FromSlash(files[i].RelativePath)))
			path = filepath.Join(itemRoot, filepath.FromSlash(files[i].RelativePath))
		}
		if err := writeDecodedDeviceLog(path, decoded); err != nil {
			return decodedCount, fmt.Errorf("write decoded device log %q: %w", files[i].RelativePath, err)
		}
		files[i] = fileMetadata(path, files[i].RelativePath)
		decodedCount++
	}
	return decodedCount, nil
}

func shouldTryDeviceLogDecode(relativePath string) bool {
	return isDeviceLogFilename(relativePath) || strings.HasPrefix(relativePath, "original/")
}

func isDeviceLogFilename(relativePath string) bool {
	name := strings.ToLower(filepath.Base(relativePath))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return name == "logfile" || strings.HasPrefix(name, "logfile_")
}

func decodeDeviceLogFile(path string) ([]byte, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false, err
	}
	if info.Size() > maxDeviceLogDecodeBytes {
		return nil, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	decoded, changed := decodeDeviceLogContent(data)
	return decoded, changed, nil
}

func writeDecodedDeviceLog(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".decoded-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func decodeDeviceLogContent(data []byte) ([]byte, bool) {
	boundary := encodedLogBoundary(data)
	if boundary < 0 {
		return data, false
	}
	decoded := append([]byte(nil), data...)
	previous := byte(boundary)
	for offset := boundary; offset < len(decoded); offset++ {
		ciphertext := data[offset]
		decoded[offset] = ciphertext ^ deviceLogXORKey[offset&7] ^ byte(offset*0x9d) ^ previous
		previous = ciphertext
	}
	return decoded, true
}

func encodedLogBoundary(data []byte) int {
	if len(data) < 1024 {
		if printableRatio(data) > 0.85 {
			return -1
		}
		return 0
	}
	if printableRatio(data[:min(len(data), 4096)]) < 0.60 {
		if decodedPrintableRatio(data, 0) >= 0.85 {
			return 0
		}
		return -1
	}
	encodedStart := -1
	for offset := len(data) - 1024; offset >= 0; offset -= 256 {
		if printableRatio(data[offset:min(len(data), offset+1024)]) < 0.60 {
			encodedStart = offset
			break
		}
	}
	if encodedStart < 0 {
		return -1
	}
	plainEnd := 0
	for offset := encodedStart; offset >= 0; offset -= 1024 {
		if printableRatio(data[offset:min(len(data), offset+1024)]) > 0.85 {
			plainEnd = offset + 1024
			break
		}
	}
	if plainEnd == 0 {
		return 0
	}
	bestBoundary := -1
	bestRatio := 0.0
	for offset := max(0, plainEnd-64); offset < min(len(data), plainEnd+1024); offset++ {
		ratio := decodedPrintableRatio(data, offset)
		if ratio > bestRatio {
			bestBoundary, bestRatio = offset, ratio
		}
	}
	if bestBoundary >= 0 && bestRatio >= 0.85 {
		return bestBoundary
	}
	return min(plainEnd, len(data))
}

func decodedPrintableRatio(data []byte, boundary int) float64 {
	end := min(len(data), boundary+1024)
	if boundary >= end {
		return 0
	}
	printable := 0
	previous := byte(boundary)
	for offset := boundary; offset < end; offset++ {
		ciphertext := data[offset]
		value := ciphertext ^ deviceLogXORKey[offset&7] ^ byte(offset*0x9d) ^ previous
		if value >= 0x20 && value <= 0x7e || value == '\n' || value == '\r' || value == '\t' {
			printable++
		}
		previous = ciphertext
	}
	return float64(printable) / float64(end-boundary)
}

func printableRatio(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	printable := 0
	for _, value := range data {
		if value >= 0x20 && value <= 0x7e || value == '\n' || value == '\r' || value == '\t' {
			printable++
		}
	}
	return float64(printable) / float64(len(data))
}
