package stock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type MarketIndex struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	ChangePercent float64 `json:"changePercent"`
	ChangeAmount  float64 `json:"changeAmount"`
	Open          float64 `json:"open"`
	PrevClose     float64 `json:"prevClose"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	Volume        int64   `json:"volume"`
	Amount        float64 `json:"amount"`
}

type MarketMoneyFlow struct {
	Date       string  `json:"date"`
	MainNet    float64 `json:"mainNet"`
	SmallNet   float64 `json:"smallNet"`
	MediumNet  float64 `json:"mediumNet"`
	LargeNet   float64 `json:"largeNet"`
	MainRatio  float64 `json:"mainRatio"`
}

type MarketSummary struct {
	Indices    []MarketIndex     `json:"indices"`
	MoneyFlow  []MarketMoneyFlow `json:"moneyFlow"`
	Date       string            `json:"date"`
}

type SectorMoneyFlow struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	MainNet   float64 `json:"mainNet"`
	MainRatio float64 `json:"mainRatio"`
	ChangePct float64 `json:"changePct"`
}

type MarketFetcher struct {
	client *http.Client
}

func NewMarketFetcher() *MarketFetcher {
	return &MarketFetcher{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (m *MarketFetcher) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
}

func (m *MarketFetcher) doGet(url string) ([]byte, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		m.setHeaders(req)
		req.Header.Set("Referer", "https://data.eastmoney.com")
		resp, err := m.client.Do(req)
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

func (m *MarketFetcher) GetSummary() (*MarketSummary, error) {
	indices := m.fetchIndices()
	moneyFlow, _ := m.fetchIndexMoneyFlow("1.000001", 10)
	if moneyFlow == nil {
		moneyFlow = []MarketMoneyFlow{}
	}
	// Use actual data date from API response, not wall clock
	dataDate := time.Now().Format("2006-01-02")
	if len(moneyFlow) > 0 && moneyFlow[0].Date != "" {
		dataDate = moneyFlow[0].Date
	}
	return &MarketSummary{
		Indices:   indices,
		MoneyFlow: moneyFlow,
		Date:      dataDate,
	}, nil
}

type indexDef struct{ code, name, tencentPrefix, emSecid string }

func (m *MarketFetcher) fetchIndices() []MarketIndex {
	indices := []indexDef{
		{"000001", "上证指数", "sh", "1.000001"},
		{"399001", "深证成指", "sz", "0.399001"},
		{"399006", "创业板指", "sz", "0.399006"},
	}

	// Try Tencent qt.gtimg.cn first
	if results := m.fetchIndicesFromTencent(indices); len(results) > 0 {
		return results
	}

	// Fallback: East Money kline API for last 2 daily bars
	return m.fetchIndicesFromEMKline(indices)
}

func (m *MarketFetcher) fetchIndicesFromTencent(indices []indexDef) []MarketIndex {
	queryParts := make([]string, len(indices))
	for i, idx := range indices {
		queryParts[i] = idx.tencentPrefix + idx.code
	}
	url := "https://qt.gtimg.cn/q=" + strings.Join(queryParts, ",")
	req, _ := http.NewRequest("GET", url, nil)
	m.setHeaders(req)
	req.Header.Set("Referer", "https://gu.qq.com")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(body), "\n")
	var results []MarketIndex
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
		codes2 := strings.Split(codePrefix, "=")
		fullCode := ""
		if len(codes2) > 0 {
			fullCode = strings.TrimPrefix(codes2[0], "v_")
		}
		price := parseFloat(parts[3])
		prevClose := parseFloat(parts[4])
		changeAmt := price - prevClose
		changePct := 0.0
		if prevClose > 0 {
			changePct = changeAmt / prevClose * 100
		}
		displayName := fullCode
		for _, idx := range indices {
			checkCode := strings.TrimPrefix(strings.TrimPrefix(fullCode, "sh"), "sz")
			if checkCode == idx.code {
				displayName = idx.name
				break
			}
		}
		results = append(results, MarketIndex{
			Code:          fullCode,
			Name:          displayName,
			Price:         price,
			ChangePercent: changePct,
			ChangeAmount:  changeAmt,
			Open:          parseFloat(parts[5]),
			PrevClose:     prevClose,
			High:          parseFloat(parts[33]),
			Low:           parseFloat(parts[34]),
			Volume:        parseInt64(parts[6]),
			Amount:        parseFloat(parts[37]),
		})
	}
	return results
}

func (m *MarketFetcher) fetchIndicesFromEMKline(indices []indexDef) []MarketIndex {
	var results []MarketIndex
	for _, idx := range indices {
		url := fmt.Sprintf("https://push2delay.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3&fields2=f51,f52,f53,f54,f55,f56,f57&klt=101&lmt=2", idx.emSecid)
		body, err := m.doGet(url)
		if err != nil {
			continue
		}
		var result struct {
			Data *struct {
				KLines []string `json:"klines"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &result); err != nil || result.Data == nil || len(result.Data.KLines) < 2 {
			continue
		}
		last := strings.Split(result.Data.KLines[len(result.Data.KLines)-1], ",")
		prev := strings.Split(result.Data.KLines[len(result.Data.KLines)-2], ",")
		if len(last) < 6 || len(prev) < 6 {
			continue
		}
		closePrice := parseFloat(last[3])
		prevClose := parseFloat(prev[3])
		changeAmt := closePrice - prevClose
		changePct := 0.0
		if prevClose > 0 {
			changePct = changeAmt / prevClose * 100
		}
		results = append(results, MarketIndex{
			Code:          idx.code,
			Name:          idx.name,
			Price:         closePrice,
			ChangePercent: changePct,
			ChangeAmount:  changeAmt,
			Open:          parseFloat(last[1]),
			PrevClose:     prevClose,
			High:          parseFloat(last[4]),
			Low:           parseFloat(last[5]),
			Volume:        parseInt64(last[6]),
			Amount:        0,
		})
	}
	if results == nil {
		results = []MarketIndex{}
	}
	return results
}

func (m *MarketFetcher) FetchIndexMoneyFlowRaw(secid string, days int) ([]MarketMoneyFlow, error) {
	return m.fetchIndexMoneyFlow(secid, days)
}

func (m *MarketFetcher) FetchSectorMoneyFlow() ([]SectorMoneyFlow, error) {
	url := "https://push2delay.eastmoney.com/api/qt/clist/get?pn=1&pz=200&np=1&fltt=2&invt=2&fs=m:90+t:2&fields=f12,f14,f2,f3,f62,f184,f66,f69,f72,f75,f78,f81,f84,f87,f204,f205,f124&ut=b2884a393a59ad64002292a3e90d46a5"

	body, err := m.doGet(url)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data *struct {
			Diff []struct {
				F12 string  `json:"f12"`
				F14 string  `json:"f14"`
				F3  float64 `json:"f3"`
				F62 float64 `json:"f62"`
				F184 float64 `json:"f184"`
			} `json:"diff"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Data == nil {
		return []SectorMoneyFlow{}, nil
	}

	var items []SectorMoneyFlow
	for _, d := range result.Data.Diff {
		// filter out board-level sectors (主板, 创业板, 科创板, 北交所, 中小板)
		if d.F14 == "主板" || d.F14 == "创业板" || d.F14 == "科创板" || d.F14 == "北交所" || d.F14 == "中小板" {
			continue
		}
		items = append(items, SectorMoneyFlow{
			Code:      d.F12,
			Name:      d.F14,
			MainNet:   d.F62,
			MainRatio: d.F184,
			ChangePct: d.F3,
		})
	}
	if items == nil {
		items = []SectorMoneyFlow{}
	}
	return items, nil
}

func (m *MarketFetcher) fetchIndexMoneyFlow(secid string, days int) ([]MarketMoneyFlow, error) {
	url := fmt.Sprintf("https://push2delay.eastmoney.com/api/qt/stock/fflow/daykline/get?secid=%s&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f62,f63&ut=b2884a393a59ad64002292a3e90d46a5&klt=101&lmt=%d", secid, days)

	body, err := m.doGet(url)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data *struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Data == nil {
		return []MarketMoneyFlow{}, nil
	}

	var items []MarketMoneyFlow
	for _, kline := range result.Data.Klines {
		parts := strings.Split(kline, ",")
		if len(parts) < 7 {
			continue
		}
		items = append(items, MarketMoneyFlow{
			Date:      parts[0],
			MainNet:   parseFloat(parts[1]),
			SmallNet:  parseFloat(parts[6]),
			MediumNet: parseFloat(parts[5]),
			LargeNet:  parseFloat(parts[2]),
			MainRatio: parseFloat(parts[3]),
		})
	}
	if items == nil {
		items = []MarketMoneyFlow{}
	}
	return items, nil
}
