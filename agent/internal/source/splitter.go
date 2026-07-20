package source

type LineSplitter struct {
	buffer []byte
	max    int
}

func NewLineSplitter(maxBytes int) *LineSplitter {
	return &LineSplitter{max: maxBytes}
}

func (s *LineSplitter) Feed(chunk []byte) []string {
	s.buffer = append(s.buffer, chunk...)
	var lines []string
	start := 0
	for i := 0; i < len(s.buffer); i++ {
		if s.buffer[i] != '\r' && s.buffer[i] != '\n' {
			continue
		}
		if s.buffer[i] == '\r' && i == len(s.buffer)-1 {
			break
		}
		end := i
		if s.buffer[i] == '\r' && i+1 < len(s.buffer) && s.buffer[i+1] == '\n' {
			i++
		}
		if end > start {
			lines = append(lines, string(s.buffer[start:end]))
		}
		start = i + 1
	}
	if start > 0 {
		s.buffer = append(s.buffer[:0], s.buffer[start:]...)
	}
	for len(s.buffer) >= s.max {
		lines = append(lines, string(s.buffer[:s.max]))
		s.buffer = append(s.buffer[:0], s.buffer[s.max:]...)
	}
	return lines
}

func (s *LineSplitter) Flush() string {
	if len(s.buffer) == 0 {
		return ""
	}
	line := string(s.buffer)
	s.buffer = s.buffer[:0]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line
}

func (s *LineSplitter) Pending() bool { return len(s.buffer) > 0 }
