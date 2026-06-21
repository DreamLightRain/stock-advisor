package stock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type EastMoneyFetcher struct {
	client *http.Client
}

type eastMoneyKLineResult struct {
	Data struct {
		KLines []string `json:"klines"`
	} `json:"data"`
}

func NewEastMoneyFetcher() *EastMoneyFetcher {
	return &EastMoneyFetcher{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (e *EastMoneyFetcher) Name() string {
	return "东方财富"
}

func (e *EastMoneyFetcher) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://quote.eastmoney.com")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
}

func (e *EastMoneyFetcher) doWithRetry(url string) ([]byte, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		e.setHeaders(req)
		resp, err := e.client.Do(req)
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
		if i < 2 {
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
	}
	return nil, fmt.Errorf("after 3 retries: %w", lastErr)
}

// GetStockIndustry fetches industry info from East Money stock detail API
func (e *EastMoneyFetcher) GetStockIndustry(code string) (string, error) {
	secid := convertToEastMoneySecID(code)
	url := fmt.Sprintf("https://push2delay.eastmoney.com/api/qt/stock/get?secid=%s&fields=f57,f58,f127", secid)
	body, err := e.doWithRetry(url)
	if err != nil {
		return "", err
	}
	var result struct {
		Data struct {
			F127 string `json:"f127"` // 所属行业
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.Data.F127, nil
}

func (e *EastMoneyFetcher) Search(keyword string) ([]SearchResult, error) {
	if keyword == "" {
		return nil, fmt.Errorf("empty keyword")
	}

	url := fmt.Sprintf("https://searchadapter.eastmoney.com/api/suggest/get?input=%s&type=14&token=D43BF722C8E33BDC906FB84D85E326E8", url.QueryEscape(keyword))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	e.setHeaders(req)
	req.Header.Set("Referer", "https://www.eastmoney.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}

	table, _ := root["QuotationCodeTable"].(map[string]interface{})
	if table == nil {
		// Try alternate format: { "data": [...] }
		if data, ok := root["data"].([]interface{}); ok {
			return parseEMItems(data)
		}
		return nil, fmt.Errorf("unexpected response format")
	}

	data, _ := table["Data"].([]interface{})
	if data == nil {
		return nil, fmt.Errorf("no data in response")
	}

	return parseEMItems(data)
}

func parseEMItems(items []interface{}) ([]SearchResult, error) {
	var results []SearchResult
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		getStr := func(keys ...string) string {
			for _, k := range keys {
				if v, ok := m[k].(string); ok && v != "" {
					return v
				}
			}
			return ""
		}

		code := getStr("Code", "code")
		name := getStr("Name", "name")
		if code == "" || name == "" {
			continue
		}

		market := ""
		if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "9") {
			market = "SH"
		} else if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") || strings.HasPrefix(code, "2") {
			market = "SZ"
		} else {
			market = "OTHER"
		}

		secType := getStr("SecurityTypeName", "SecurityType", "Type", "type")
		stockType := "A股"
		if secType != "" {
			stockType = secType
		}

		industry := getStr("Industry", "industry", "HYName", "hyName")

		results = append(results, SearchResult{
			Code:     code,
			Name:     name,
			FullCode: strings.ToLower(market) + code,
			Market:   market,
			Type:     stockType,
			Industry: industry,
		})
	}
	return results, nil
}

func (e *EastMoneyFetcher) FetchKLine(code string, days int) ([]KLine, error) {
	if days <= 0 {
		days = 100
	}
	if days > 500 {
		days = 500
	}

	secid := convertToEastMoneySecID(code)
	url := fmt.Sprintf(
		"https://push2delay.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61&klt=101&fqt=1&end=20500101&lmt=%d",
		secid, days,
	)

	body, err := e.doWithRetry(url)
	if err != nil {
		return nil, err
	}

	var result eastMoneyKLineResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Data.KLines == nil {
		return nil, fmt.Errorf("no kline data for %s", code)
	}

	klines := make([]KLine, 0, len(result.Data.KLines))
	for _, klineStr := range result.Data.KLines {
		parts := strings.Split(klineStr, ",")
		if len(parts) < 11 {
			continue
		}

		kline := KLine{
			Date:   parts[0],
			Open:   parseFloat(parts[1]),
			Close:  parseFloat(parts[2]),
			High:   parseFloat(parts[3]),
			Low:    parseFloat(parts[4]),
			Volume: parseInt64(parts[5]),
			Amount: parseFloat(parts[6]),
		}
		klines = append(klines, kline)
	}

	return klines, nil
}

func (e *EastMoneyFetcher) FetchTimeSharing(code string) ([]KLine, error) {
	secid := convertToEastMoneySecID(code)
	url := fmt.Sprintf(
		"https://push2delay.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3&fields2=f51,f52,f53,f54,f55,f56,f57&klt=1&fqt=1&lmt=244&end=20500101",
		secid,
	)
	body, err := e.doWithRetry(url)
	if err != nil {
		return nil, err
	}
	var result eastMoneyKLineResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Data.KLines == nil {
		return nil, fmt.Errorf("no timesharing data for %s", code)
	}
	klines := make([]KLine, 0, len(result.Data.KLines))
	for _, klineStr := range result.Data.KLines {
		parts := strings.Split(klineStr, ",")
		if len(parts) < 7 {
			continue
		}
		klines = append(klines, KLine{
			Date:   parts[0],
			Open:   parseFloat(parts[1]),
			Close:  parseFloat(parts[2]),
			High:   parseFloat(parts[3]),
			Low:    parseFloat(parts[4]),
			Volume: parseInt64(parts[5]),
			Amount: parseFloat(parts[6]),
		})
	}
	return klines, nil
}

func (e *EastMoneyFetcher) FetchMoneyFlow(code string, days int) ([]MoneyFlowItem, error) {
	if days <= 0 {
		days = 10
	}
	if days > 120 {
		days = 120
	}
	secid := convertToEastMoneySecID(code)
	url := fmt.Sprintf(
		"https://push2delay.eastmoney.com/api/qt/stock/fflow/daykline/get?secid=%s&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63&ut=b2884a393a59ad64002292a3e90d46a5&klt=101&lmt=%d",
		secid, days,
	)
	body, err := e.doWithRetry(url)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			KLines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Data.KLines == nil {
		return []MoneyFlowItem{}, nil
	}
	items := make([]MoneyFlowItem, 0, len(result.Data.KLines))
	for _, line := range result.Data.KLines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}
		item := MoneyFlowItem{
			Date:         parts[0], // f51: date
			MainNet:      parseFloat(parts[1]),  // f52: 主力净流入额
			SmallNet:     parseFloat(parts[2]),  // f53: 小单净流入额
			MediumNet:    parseFloat(parts[3]),  // f54: 中单净流入额
			LargeNet:     parseFloat(parts[4]),  // f55: 大单净流入额
			SuperLargeNet: parseFloat(parts[5]), // f56: 超大单净流入额
		}
		if len(parts) >= 7 {
			item.MainRatio = parseFloat(parts[6]) // f57: 主力净流入占比
		}
		if len(parts) >= 12 {
			item.Close = parseFloat(parts[11]) // f62: 收盘价
		}
		items = append(items, item)
	}
	return items, nil
}

func convertToEastMoneySecID(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "bj") {
		if strings.HasPrefix(code, "sh") {
			return "1." + code[2:]
		} else if strings.HasPrefix(code, "sz") {
			return "0." + code[2:]
		} else {
			return "2." + code[2:]
		}
	}
	if strings.HasPrefix(code, "6") {
		return "1." + code
	}
	if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") || strings.HasPrefix(code, "2") {
		return "0." + code
	}
	return "1." + code
}
