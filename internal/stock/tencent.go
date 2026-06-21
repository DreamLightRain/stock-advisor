package stock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TencentFetcher struct {
	client *http.Client
}

func NewTencentFetcher() *TencentFetcher {
	return &TencentFetcher{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (t *TencentFetcher) Name() string {
	return "腾讯财经"
}

func (t *TencentFetcher) FetchRealTime(codes []string) (map[string]*RealTimeData, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("empty codes")
	}

	queryCodes := make([]string, len(codes))
	for i, c := range codes {
		queryCodes[i] = convertToTencentCode(c)
	}

	url := "https://qt.gtimg.cn/q=" + strings.Join(queryCodes, ",")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://gu.qq.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseTencentResponse(string(body))
}

func (t *TencentFetcher) FetchKLine(code string, days int) ([]KLine, error) {
	if days <= 0 {
		days = 100
	}
	if days > 500 {
		days = 500
	}
	queryCode := convertToTencentCode(code)
	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,day,,,%d,qfq", queryCode, days)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://finance.qq.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data map[string]struct {
			QfqDay [][]interface{} `json:"qfqday"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	stockData, ok := result.Data[queryCode]
	if !ok || stockData.QfqDay == nil {
		return nil, fmt.Errorf("no tencent kline data for %s", code)
	}
	klines := make([]KLine, 0, len(stockData.QfqDay))
	for _, item := range stockData.QfqDay {
		if len(item) < 6 {
			continue
		}
		kline := KLine{
			Date:   toString(item[0]),
			Open:   toFloat(item[1]),
			Close:  toFloat(item[2]),
			High:   toFloat(item[3]),
			Low:    toFloat(item[4]),
			Volume: toInt64(item[5]),
		}
		klines = append(klines, kline)
	}
	return klines, nil
}

func toString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	}
	return 0
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case string:
		var i int64
		fmt.Sscanf(val, "%d", &i)
		return i
	}
	return 0
}

func convertToTencentCode(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") {
		return code
	}
	if strings.HasPrefix(code, "6") {
		return "sh" + code
	}
	if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") || strings.HasPrefix(code, "2") {
		return "sz" + code
	}
	return code
}

func parseTencentResponse(data string) (map[string]*RealTimeData, error) {
	result := make(map[string]*RealTimeData)
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		start := strings.Index(line, "\"")
		end := strings.LastIndex(line, "\"")
		if start < 0 || end <= start {
			continue
		}
		content := line[start+1 : end]
		parts := strings.Split(content, "~")

		if len(parts) < 40 {
			continue
		}

		codePrefix := strings.TrimSpace(line[:start])
		codes := strings.Split(codePrefix, "=")
		fullCode := ""
		if len(codes) > 0 {
			fullCode = strings.TrimPrefix(codes[0], "v_")
		}

		item := &RealTimeData{
			Code:        fullCode,
			Name:        parts[1],
			Price:       parseFloat(parts[3]),
			PrevClose:   parseFloat(parts[4]),
			Open:        parseFloat(parts[5]),
			Volume:      parseInt64(parts[6]),
			Bid1:        parseFloat(parts[9]),
			Ask1:        parseFloat(parts[10]),
			High:        parseFloat(parts[33]),
			Low:         parseFloat(parts[34]),
			Amount:      parseFloat(parts[37]),
			UpdateTime:  parts[31],
		}

		if item.Price > 0 && item.PrevClose > 0 {
			item.ChangeAmount = item.Price - item.PrevClose
			item.ChangePercent = (item.ChangeAmount / item.PrevClose) * 100
		}

		result[item.Code] = item
	}

	return result, nil
}
