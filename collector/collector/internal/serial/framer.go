package serial

import "time"

type IdleGapFramer struct {
	idleGap       time.Duration
	maxFrameBytes int
	buffer        []byte
	lastByteAt    time.Time
}

func NewIdleGapFramer(idleGap time.Duration, maxFrameBytes int) *IdleGapFramer {
	if idleGap <= 0 {
		idleGap = 10 * time.Millisecond
	}
	if maxFrameBytes <= 0 {
		maxFrameBytes = 10 * 1024
	}
	return &IdleGapFramer{idleGap: idleGap, maxFrameBytes: maxFrameBytes}
}

// Push copies chunk before returning. Returned frames also own their storage.
func (f *IdleGapFramer) Push(at time.Time, chunk []byte) [][]byte {
	if len(chunk) == 0 {
		return nil
	}
	var frames [][]byte
	if len(f.buffer) > 0 && !f.lastByteAt.IsZero() && at.Sub(f.lastByteAt) >= f.idleGap {
		frames = append(frames, f.take())
	}
	for len(chunk) > 0 {
		remaining := f.maxFrameBytes - len(f.buffer)
		if remaining > len(chunk) {
			remaining = len(chunk)
		}
		f.buffer = append(f.buffer, chunk[:remaining]...)
		chunk = chunk[remaining:]
		f.lastByteAt = at
		if len(f.buffer) == f.maxFrameBytes {
			frames = append(frames, f.take())
		}
	}
	return frames
}

func (f *IdleGapFramer) FlushIfIdle(now time.Time) ([]byte, bool) {
	if len(f.buffer) == 0 || f.lastByteAt.IsZero() || now.Sub(f.lastByteAt) < f.idleGap {
		return nil, false
	}
	return f.take(), true
}

func (f *IdleGapFramer) Flush() ([]byte, bool) {
	if len(f.buffer) == 0 {
		return nil, false
	}
	return f.take(), true
}

func (f *IdleGapFramer) PendingBytes() int { return len(f.buffer) }

func (f *IdleGapFramer) take() []byte {
	frame := append([]byte(nil), f.buffer...)
	f.buffer = f.buffer[:0]
	f.lastByteAt = time.Time{}
	return frame
}
