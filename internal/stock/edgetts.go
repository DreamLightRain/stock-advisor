package stock

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type EdgeTTS struct {
	voice   string
	rate    string
	volume  string
	client  *http.Client
	token   string
	tokenMu sync.Mutex
	tokenAt time.Time
}

var defaultVoice = "zh-CN-XiaoxiaoNeural"

func NewEdgeTTS(voice string) *EdgeTTS {
	if voice == "" {
		voice = defaultVoice
	}
	return &EdgeTTS{
		voice:  voice,
		rate:   "+5%",
		volume: "+0%",
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (e *EdgeTTS) getToken() (string, error) {
	e.tokenMu.Lock()
	defer e.tokenMu.Unlock()
	if e.token != "" && time.Since(e.tokenAt) < 5*time.Minute {
		return e.token, nil
	}
	req, _ := http.NewRequest("GET", "https://edge.microsoft.com/translate/auth", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("edge token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("edge token read: %w", err)
	}
	token := strings.TrimSpace(string(body))
	if token == "" {
		return "", fmt.Errorf("edge token: empty response")
	}
	e.token = token
	e.tokenAt = time.Now()
	return token, nil
}

func (e *EdgeTTS) Synthesize(text string) ([]byte, error) {
	token, err := e.getToken()
	if err != nil {
		return nil, err
	}

	connID := uuid.New().String()
	u := fmt.Sprintf("wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=%s&ConnectionId=%s",
		url.QueryEscape(token), connID)

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	header := http.Header{}
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	header.Set("Origin", "https://edge.microsoft.com")
	header.Set("Pragma", "no-cache")

	conn, _, err := dialer.Dial(u, header)
	if err != nil {
		return nil, fmt.Errorf("edge ws dial: %w", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	reqID := uuid.New().String()
	ssml := fmt.Sprintf(`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xmlns:mstts='https://www.w3.org/2001/mstts' xml:lang='zh-CN'><voice name='%s'><prosody rate='%s' pitch='+0Hz' volume='%s'>%s</prosody></voice></speak>`,
		e.voice, e.rate, e.volume, escapeXML(text))

	msg := fmt.Sprintf("X-RequestId:%s\r\nContent-Type:application/ssml+xml\r\nX-Timestamp:%sZ\r\nPath:speech\r\n\r\n%s",
		reqID, time.Now().UTC().Format("2006-01-02T15:04:05"), ssml)

	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return nil, fmt.Errorf("edge ws write: %w", err)
	}

	var audio []byte
	turnEnd := []byte("Path:turn.end")

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Check for binary audio data (starts after headers)
		if len(data) >= 2 && data[0] != 'X' && data[0] != 'P' && data[0] != 'C' {
			// Find the double newline that separates headers from body
			idx := indexOf(data, []byte("\r\n\r\n"))
			if idx > 0 {
				body := data[idx+4:]
				if len(body) > 0 {
					audio = append(audio, body...)
				}
				// Check if this is a turn end message
				if contains(data[:idx], turnEnd) {
					break
				}
			}
		}

		// Check text messages for turn end
		if contains(data, turnEnd) {
			break
		}
	}

	if len(audio) == 0 {
		return nil, fmt.Errorf("edge tts: no audio received")
	}
	return audio, nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func indexOf(data, sub []byte) int {
	for i := 0; i <= len(data)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if data[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func contains(data, sub []byte) bool {
	return indexOf(data, sub) >= 0
}

// EdgeTTSVoices returns available Chinese voices
func EdgeTTSVoices() []string {
	return []string{
		"zh-CN-XiaoxiaoNeural",  // 温柔女声 (推荐)
		"zh-CN-YunyangNeural",   // 沉稳男声 (财经播报)
		"zh-CN-YunxiNeural",     // 自然男声
		"zh-CN-YunjianNeural",   // 自信男声
		"zh-CN-XiaoyiNeural",    // 活泼女声
		"zh-CN-YunzeNeural",     // 少年音
		"zh-CN-XiaohanNeural",   // 温婉女声
		"zh-CN-XiaomengNeural",  // 可爱女声
	}
}

func EncodeAudioBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
