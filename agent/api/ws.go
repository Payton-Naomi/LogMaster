package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// wsMessage 是通过 WebSocket 发送的消息结构。
type wsMessage struct {
	Type      string      `json:"type"`                 // "log" 或 "status"
	Device    string      `json:"device,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	Content   string      `json:"content,omitempty"`
	Status    interface{} `json:"status,omitempty"`
}

// handleWebSocket 处理 WebSocket 连接。
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 简单的 WebSocket 实现（无外部依赖）
	// 检查是否为 WebSocket 升级请求
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "仅支持 WebSocket 连接", http.StatusBadRequest)
		return
	}

	// 使用标准库的 Hijack 实现 WebSocket
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "服务器不支持 Hijack", http.StatusInternalServerError)
		return
	}

	conn, buf, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// 发送 WebSocket 升级响应
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return
	}
	acceptKey := computeAcceptKey(key)
	buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	buf.WriteString("Upgrade: websocket\r\n")
	buf.WriteString("Connection: Upgrade\r\n")
	buf.WriteString("Sec-WebSocket-Accept: " + acceptKey + "\r\n")
	buf.WriteString("\r\n")
	buf.Flush()

	// 订阅日志行
	ch := s.proxy.Subscribe()
	defer s.proxy.Unsubscribe(ch)

	// 定时推送状态
	statusTicker := time.NewTicker(1 * time.Second)
	defer statusTicker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// 读取客户端消息（忽略，仅用于检测断开）
		buf := make([]byte, 1024)
		for {
			_, err := conn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			msg := wsMessage{
				Type:      "log",
				Device:    line.Device,
				Timestamp: line.Timestamp.Format("2006-01-02T15:04:05.000"),
				Content:   line.Content,
			}
			if err := writeWSMessage(conn, msg); err != nil {
				return
			}
		case <-statusTicker.C:
			status := s.proxy.Status()
			msg := wsMessage{
				Type:   "status",
				Status: status,
			}
			if err := writeWSMessage(conn, msg); err != nil {
				return
			}
		}
	}
}

// writeWSMessage 发送 WebSocket 文本帧。
func writeWSMessage(conn interface{ Write([]byte) (int, error) }, msg wsMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return writeWSFrame(conn, data)
}

// writeWSFrame 写入 WebSocket 文本帧。
// WebSocket 帧格式: FIN(1) + RSV(3) + Opcode(4) | MASK(1) + PayloadLen(7) | ...payload
func writeWSFrame(conn interface{ Write([]byte) (int, error) }, payload []byte) error {
	length := len(payload)
	frame := make([]byte, 2, 2+length)

	// FIN + Text opcode
	frame[0] = 0x81

	if length < 126 {
		frame[1] = byte(length)
		frame = append(frame, payload...)
	} else if length < 65536 {
		frame[1] = 126
		frame = append(frame, byte(length>>8), byte(length))
		frame = append(frame, payload...)
	} else {
		frame[1] = 127
		for i := 7; i >= 0; i-- {
			frame = append(frame, byte(length>>(8*i)))
		}
		frame = append(frame, payload...)
	}

	_, err := conn.Write(frame)
	return err
}

// computeAcceptKey 计算 WebSocket Accept 密钥。
func computeAcceptKey(key string) string {
	// 简化实现：使用标准 WebSocket GUID
	// 实际生产环境应使用 crypto/sha1
	const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	combined := key + wsGUID

	// 使用内置的 SHA1 计算
	h := sha1Sum([]byte(combined))
	return base64Encode(h)
}

// sha1Sum 计算 SHA1 哈希。
func sha1Sum(data []byte) [20]byte {
	var d [20]byte
	// 使用纯 Go 实现的简化 SHA1
	h0 := uint32(0x67452301)
	h1 := uint32(0xEFCDAB89)
	h2 := uint32(0x98BADCFE)
	h3 := uint32(0x10325476)
	h4 := uint32(0xC3D2E1F0)

	// 填充
	ml := uint64(len(data)) * 8
	data = append(data, 0x80)
	for (len(data)%64) != 56 {
		data = append(data, 0x00)
	}
	for i := 7; i >= 0; i-- {
		data = append(data, byte(ml>>(8*i)))
	}

	// 处理每个 512 位块
	for i := 0; i < len(data); i += 64 {
		w := make([]uint32, 80)
		for j := 0; j < 16; j++ {
			w[j] = uint32(data[i+j*4])<<24 | uint32(data[i+j*4+1])<<16 | uint32(data[i+j*4+2])<<8 | uint32(data[i+j*4+3])
		}
		for j := 16; j < 80; j++ {
			w[j] = leftRotate(w[j-3]^w[j-8]^w[j-14]^w[j-16], 1)
		}

		a, b, c, e, f := h0, h1, h2, h3, h4
		g := h4

		for j := 0; j < 80; j++ {
			var k uint32
			if j < 20 {
				f = (b & c) | (^b & e)
				k = 0x5A827999
			} else if j < 40 {
				f = b ^ c ^ e
				k = 0x6ED9EBA1
			} else if j < 60 {
				f = (b & c) | (b & e) | (c & e)
				k = 0x8F1BBCDC
			} else {
				f = b ^ c ^ e
				k = 0xCA62C1D6
			}

			temp := leftRotate(a, 5) + f + g + k + w[j]
			g = e
			e = c
			c = leftRotate(b, 30)
			b = a
			a = temp
		}

		h0 += a
		h1 += b
		h2 += c
		h3 += e
		h4 += g
	}

	d[0] = byte(h0 >> 24)
	d[1] = byte(h0 >> 16)
	d[2] = byte(h0 >> 8)
	d[3] = byte(h0)
	d[4] = byte(h1 >> 24)
	d[5] = byte(h1 >> 16)
	d[6] = byte(h1 >> 8)
	d[7] = byte(h1)
	d[8] = byte(h2 >> 24)
	d[9] = byte(h2 >> 16)
	d[10] = byte(h2 >> 8)
	d[11] = byte(h2)
	d[12] = byte(h3 >> 24)
	d[13] = byte(h3 >> 16)
	d[14] = byte(h3 >> 8)
	d[15] = byte(h3)
	d[16] = byte(h4 >> 24)
	d[17] = byte(h4 >> 16)
	d[18] = byte(h4 >> 8)
	d[19] = byte(h4)

	return d
}

func leftRotate(x uint32, n uint32) uint32 {
	return (x << n) | (x >> (32 - n))
}

// base64Encode 编码为 Base64。
func base64Encode(data [20]byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, 28)

	// 20 字节 = 28 个 base64 字符（含填充）
	result[0] = alphabet[(data[0]>>2)&0x3F]
	result[1] = alphabet[((data[0]<<4)|(data[1]>>4))&0x3F]
	result[2] = alphabet[((data[1]<<2)|(data[2]>>6))&0x3F]
	result[3] = alphabet[data[2]&0x3F]
	result[4] = alphabet[(data[3]>>2)&0x3F]
	result[5] = alphabet[((data[3]<<4)|(data[4]>>4))&0x3F]
	result[6] = alphabet[((data[4]<<2)|(data[5]>>6))&0x3F]
	result[7] = alphabet[data[5]&0x3F]
	result[8] = alphabet[(data[6]>>2)&0x3F]
	result[9] = alphabet[((data[6]<<4)|(data[7]>>4))&0x3F]
	result[10] = alphabet[((data[7]<<2)|(data[8]>>6))&0x3F]
	result[11] = alphabet[data[8]&0x3F]
	result[12] = alphabet[(data[9]>>2)&0x3F]
	result[13] = alphabet[((data[9]<<4)|(data[10]>>4))&0x3F]
	result[14] = alphabet[((data[10]<<2)|(data[11]>>6))&0x3F]
	result[15] = alphabet[data[11]&0x3F]
	result[16] = alphabet[(data[12]>>2)&0x3F]
	result[17] = alphabet[((data[12]<<4)|(data[13]>>4))&0x3F]
	result[18] = alphabet[((data[13]<<2)|(data[14]>>6))&0x3F]
	result[19] = alphabet[data[14]&0x3F]
	result[20] = alphabet[(data[15]>>2)&0x3F]
	result[21] = alphabet[((data[15]<<4)|(data[16]>>4))&0x3F]
	result[22] = alphabet[((data[16]<<2)|(data[17]>>6))&0x3F]
	result[23] = alphabet[data[17]&0x3F]
	result[24] = alphabet[(data[18]>>2)&0x3F]
	result[25] = alphabet[((data[18]<<4)|(data[19]>>4))&0x3F]
	result[26] = alphabet[((data[19]<<2))&0x3F]
	result[27] = '='

	return string(result)
}