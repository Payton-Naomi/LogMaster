package segment

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Recover seals interrupted temporary segments and redelivers every completed
// segment for one device. The delivery callback must be idempotent.
func Recover(ctx context.Context, config Config, deliver Deliver) (int, error) {
	if deliver == nil {
		return 0, errors.New("segment recovery delivery callback is required")
	}
	prefix := SanitizeFilename(config.DeviceSN) + "_" + SanitizeFilename(config.PortName) + "_"
	entries, err := os.ReadDir(config.Directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read segment directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".log.tmp") {
			continue
		}
		if _, err := sealInterrupted(filepath.Join(config.Directory, entry.Name())); err != nil {
			return 0, err
		}
	}
	entries, err = os.ReadDir(config.Directory)
	if err != nil {
		return 0, err
	}
	recovered := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return recovered, err
		}
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		metadata, err := inspectCompleted(filepath.Join(config.Directory, entry.Name()), config.DeviceSN, config.PortName)
		if err != nil {
			return recovered, err
		}
		if err := deliver(ctx, metadata); err != nil {
			return recovered, fmt.Errorf("redeliver segment %s: %w", metadata.Path, err)
		}
		recovered++
	}
	return recovered, nil
}

func sealInterrupted(path string) (string, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	reader := bufio.NewReader(file)
	var completeBytes int64
	var lines int64
	for {
		line, readErr := reader.ReadString('\n')
		if strings.HasSuffix(line, "\n") {
			completeBytes += int64(len(line))
			lines++
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				file.Close()
				return "", readErr
			}
			break
		}
	}
	if lines == 0 {
		file.Close()
		return "", fmt.Errorf("interrupted segment %s contains no complete entries", path)
	}
	if err := file.Truncate(completeBytes); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(path), ".log.tmp")
	lastUnderscore := strings.LastIndexByte(base, '_')
	if lastUnderscore < 0 {
		return "", fmt.Errorf("invalid temporary segment name %s", path)
	}
	first, err := strconv.ParseInt(base[lastUnderscore+1:], 10, 64)
	if err != nil {
		return "", fmt.Errorf("parse temporary segment sequence: %w", err)
	}
	finalPath := filepath.Join(filepath.Dir(path), fmt.Sprintf("%s_%d-%d.log", base[:lastUnderscore], first, first+lines-1))
	if err := os.Rename(path, finalPath); err != nil {
		return "", err
	}
	return finalPath, nil
}

func inspectCompleted(path, deviceSN, portName string) (Completed, error) {
	file, err := os.Open(path)
	if err != nil {
		return Completed{}, err
	}
	h := sha256.New()
	size, err := io.Copy(h, file)
	closeErr := file.Close()
	if err != nil {
		return Completed{}, err
	}
	if closeErr != nil {
		return Completed{}, closeErr
	}
	base := strings.TrimSuffix(filepath.Base(path), ".log")
	lastUnderscore := strings.LastIndexByte(base, '_')
	dash := strings.LastIndexByte(base, '-')
	if lastUnderscore < 0 || dash < lastUnderscore {
		return Completed{}, fmt.Errorf("invalid completed segment name %s", path)
	}
	first, err := strconv.ParseInt(base[lastUnderscore+1:dash], 10, 64)
	if err != nil {
		return Completed{}, err
	}
	last, err := strconv.ParseInt(base[dash+1:], 10, 64)
	if err != nil {
		return Completed{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Completed{}, err
	}
	return Completed{Path: path, SHA256: fmt.Sprintf("%x", h.Sum(nil)), SizeBytes: size, DeviceSN: deviceSN, PortName: portName, FirstSequence: first, LastSequence: last, CreatedAt: info.ModTime(), CompletedAt: time.Now().UTC()}, nil
}
