package stock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SinaFetcher struct {
	client *http.Client
	rl     *RateLimiter
}

func NewSinaFetcher() *SinaFetcher {
	return &SinaFetcher{
		client: &http.Client{Timeout: 5 * time.Second},
		rl:     NewRateLimiter(300 * time.Millisecond),
	}
}

func (s *SinaFetcher) Name() string {
	return "新浪财经"
}

func (s *SinaFetcher) doGet(url string) ([]byte, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		s.rl.Wait()
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Referer", "https://finance.sina.com.cn")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		resp, err := s.client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil {
				return body, nil
			}
			lastErr = readErr
		} else {
			lastErr = err
		}
		doWithBackoff(i)
	}
	return nil, fmt.Errorf("after 3 retries: %w", lastErr)
}

func (s *SinaFetcher) FetchRealTime(codes []string) (map[string]*RealTimeData, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("empty codes")
	}

	queryCodes := make([]string, len(codes))
	for i, c := range codes {
		queryCodes[i] = convertToSinaCode(c)
	}

	url := "https://hq.sinajs.cn/list=" + strings.Join(queryCodes, ",")
	body, err := s.doGet(url)
	if err != nil {
		return nil, err
	}

	return parseSinaResponse(string(body))
}

func convertToSinaCode(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "bj") {
		return code
	}
	if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "9") {
		return "sh" + code
	}
	if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") || strings.HasPrefix(code, "2") {
		return "sz" + code
	}
	if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") {
		return "bj" + code
	}
	return code
}

func parseSinaResponse(data string) (map[string]*RealTimeData, error) {
	result := make(map[string]*RealTimeData)
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		idx := strings.Index(line, "\"")
		if idx < 0 {
			continue
		}
		prefix := line[:idx]
		content := line[idx+1:]
		if strings.HasSuffix(content, "\"") {
			content = content[:len(content)-1]
		}

		codeStart := strings.Index(prefix, "hq_str_")
		if codeStart < 0 {
			continue
		}
		code := strings.TrimPrefix(prefix[codeStart:], "hq_str_")

		parts := strings.Split(content, ",")
		if len(parts) < 32 {
			continue
		}

		item := &RealTimeData{
			Code:    code,
			Name:    parts[0],
			Open:    parseFloat(parts[1]),
			PrevClose: parseFloat(parts[2]),
			Price:   parseFloat(parts[3]),
			High:    parseFloat(parts[4]),
			Low:     parseFloat(parts[5]),
			Bid1:    parseFloat(parts[6]),
			Ask1:    parseFloat(parts[7]),
			Volume:  parseInt64(parts[8]),
			Amount:  parseFloat(parts[9]),
		}

		if item.Price > 0 && item.PrevClose > 0 {
			item.ChangeAmount = item.Price - item.PrevClose
			item.ChangePercent = (item.ChangeAmount / item.PrevClose) * 100
		}

		if len(parts) > 31 {
			item.UpdateTime = parts[31] + " " + parts[32]
		}

		result[item.Code] = item
	}

	return result, nil
}

func (s *SinaFetcher) FetchMoneyFlow(code string, days int) ([]MoneyFlowItem, error) {
	if days <= 0 {
		days = 10
	}
	if days > 120 {
		days = 120
	}

	sinaCode := convertToSinaCode(code)
	url := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/api/json_v2.php/MoneyFlow/ssi_jsq?daima=%s&page=1&num=%d", sinaCode, days)
	body, err := s.doGet(url)
	if err != nil {
		return nil, err
	}

	// Try JSON array format
	var records []map[string]interface{}
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, err
	}

	items := make([]MoneyFlowItem, 0, len(records))
	for _, r := range records {
		date, _ := r["date"].(string)
		if date == "" {
			date, _ = r["d"].(string)
		}
		if date == "" {
			continue
		}
		if idx := strings.Index(date, " "); idx > 0 {
			date = date[:idx]
		}

		toFloat := func(key string, altKeys ...string) float64 {
			if v, ok := r[key].(float64); ok {
				return v
			}
			if s, ok := r[key].(string); ok {
				f, _ := strconv.ParseFloat(s, 64)
				return f
			}
			for _, ak := range altKeys {
				if v, ok := r[ak].(float64); ok {
					return v
				}
				if s, ok := r[ak].(string); ok {
					f, _ := strconv.ParseFloat(s, 64)
					return f
				}
			}
			return 0
		}

		items = append(items, MoneyFlowItem{
			Date:         date,
			MainNet:      toFloat("main_net", "net_amount_main"),
			SmallNet:     toFloat("small_net", "net_amount_small"),
			MediumNet:    toFloat("medium_net", "net_amount_mid"),
			LargeNet:     toFloat("big_net", "net_amount_big"),
			SuperLargeNet: toFloat("super_big_net", "net_amount_super_big"),
			MainRatio:    toFloat("main_net_pct_xto", "net_amount_main_ratio"),
			Close:        toFloat("close", "trade"),
		})
	}
	if items == nil {
		items = []MoneyFlowItem{}
	}
	return items, nil
}

func parseFloat(s string) float64 {
	var v float64
	fmt.Sscanf(strings.TrimSpace(s), "%f", &v)
	return v
}

func parseInt64(s string) int64 {
	var v int64
	fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	return v
}
