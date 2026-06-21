package stock

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type DataCenterFetcher struct {
	client *http.Client
}

func NewDataCenterFetcher() *DataCenterFetcher {
	return &DataCenterFetcher{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DataCenterFetcher) Name() string {
	return "东方数据中心"
}

func (d *DataCenterFetcher) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://data.eastmoney.com")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
}

func (d *DataCenterFetcher) doGet(url string) ([]byte, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		d.setHeaders(req)
		resp, err := d.client.Do(req)
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

// dcRecord is a flexible money flow record that accepts both number and string fields
type dcRecord map[string]interface{}

type dcResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func dcGetString(r dcRecord, key string) string {
	if v, ok := r[key].(string); ok {
		return v
	}
	return ""
}

func dcGetFloat(r dcRecord, key string) float64 {
	switch v := r[key].(type) {
	case float64:
		return v
	case string:
		if v == "" || v == "-" {
			return 0
		}
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return f
		}
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

func convertCodeToDCSecCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.TrimPrefix(code, "sh")
	code = strings.TrimPrefix(code, "sz")
	code = strings.TrimPrefix(code, "bj")
	if strings.HasPrefix(code, "6") {
		return code + ".SH"
	}
	return code + ".SZ"
}

func (d *DataCenterFetcher) FetchMoneyFlow(code string, days int) ([]MoneyFlowItem, error) {
	if days <= 0 {
		days = 10
	}
	if days > 120 {
		days = 120
	}

	secCode := convertCodeToDCSecCode(code)
	apiURL := fmt.Sprintf(
		"https://datacenter.eastmoney.com/api/data/v1/get?reportName=RPT_MONEY_FLOW_STOCK_IN_DETAIL&columns=ALL&filter=(SECUCODE%%3D%%22%s%%22)&pageNumber=1&pageSize=%d&sortTypes=-1&sortCol=TRADE_DATE&source=WEB",
		secCode, days,
	)

	body, err := d.doGet(apiURL)
	if err != nil {
		log.Printf("[数据中心] HTTP error for %s: %v", code, err)
		return nil, err
	}

	// Try nested format first: {"data": {"list": [...]}}
	var nestedResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    *struct {
			Total int            `json:"total"`
			List  []dcRecord     `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &nestedResp); err == nil && nestedResp.Code == 0 && nestedResp.Data != nil && len(nestedResp.Data.List) > 0 {
		return dcRecordsToItems(nestedResp.Data.List), nil
	}

	// Try flat array format: {"data": [...]}
	var flatResp struct {
		Code    int        `json:"code"`
		Message string     `json:"message"`
		Data    []dcRecord `json:"data"`
	}
	if err := json.Unmarshal(body, &flatResp); err == nil && flatResp.Code == 0 && len(flatResp.Data) > 0 {
		return dcRecordsToItems(flatResp.Data), nil
	}

	// Parse raw data for diagnostics
	var raw dcResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("datacenter parse error: %w", err)
	}
	if raw.Code != 0 {
		return nil, fmt.Errorf("datacenter API error: code=%d msg=%s", raw.Code, raw.Message)
	}

	return []MoneyFlowItem{}, nil
}

func dcRecordsToItems(records []dcRecord) []MoneyFlowItem {
	items := make([]MoneyFlowItem, 0, len(records))
	for _, r := range records {
		dateStr := dcGetString(r, "TRADE_DATE")
		if dateStr == "" {
			dateStr = dcGetString(r, "DATE")
		}
		if dateStr == "" {
			continue
		}
		if idx := strings.Index(dateStr, " "); idx > 0 {
			dateStr = dateStr[:idx]
		}

		mainNet := dcGetFloat(r, "TOTAL_MAIN_NET_INFLOW")
		if mainNet == 0 {
			mainNet = dcGetFloat(r, "NET_AMOUNT_MAIN") // alternative field name
		}

		items = append(items, MoneyFlowItem{
			Date:         dateStr,
			MainNet:      mainNet,
			SmallNet:     dcGetFloat(r, "RETAIL_NET_INFLOW"),
			MediumNet:    dcGetFloat(r, "MEDIUM_NET_INFLOW"),
			LargeNet:     dcGetFloat(r, "LARGE_NET_INFLOW"),
			SuperLargeNet: dcGetFloat(r, "SUPER_LARGE_NET_INFLOW"),
			MainRatio:    dcGetFloat(r, "MAIN_NET_INFLOW_RATIO"),
			Close:        dcGetFloat(r, "CLOSE_PRICE"),
		})
	}
	if items == nil {
		items = []MoneyFlowItem{}
	}
	return items
}

// FetchSectorMoneyFlowByDate attempts to get historical sector (industry) money flow for a specific date.
// Uses the datacenter API first, falls back to per-sector daykline query via push2delay.
func (d *DataCenterFetcher) FetchSectorMoneyFlowByDate(date string) ([]SectorMoneyFlow, error) {
	reportNames := []string{
		"RPT_MONEY_FLOW_INDUSTRY",
		"RPT_INDUSTRY_MONEY_FLOW",
		"RPT_INDUSTRY_FLOW",
	}
	for _, report := range reportNames {
		apiURL := fmt.Sprintf(
			"https://datacenter.eastmoney.com/api/data/v1/get?reportName=%s&columns=ALL&filter=(TRADE_DATE='%s')&pageNumber=1&pageSize=50&sortTypes=-1&sortCol=NET_MF_AMOUNT&source=WEB",
			report, date,
		)
		body, err := d.doGet(apiURL)
		if err != nil {
			continue
		}
		var nestedResp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    *struct {
				Total int        `json:"total"`
				List  []dcRecord `json:"list"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &nestedResp); err != nil || nestedResp.Data == nil || len(nestedResp.Data.List) == 0 {
			continue
		}
		if nestedResp.Code != 0 {
			continue
		}
		items := make([]SectorMoneyFlow, 0, len(nestedResp.Data.List))
		for _, r := range nestedResp.Data.List {
			code := dcGetString(r, "INDUSTRY_CODE")
			if code == "" {
				code = dcGetString(r, "CODE")
			}
			name := dcGetString(r, "INDUSTRY_NAME")
			if name == "" {
				name = dcGetString(r, "NAME")
			}
			if name == "" {
				continue
			}
			mainNet := dcGetFloat(r, "NET_MF_AMOUNT")
			if mainNet == 0 {
				mainNet = dcGetFloat(r, "MAIN_NET_INFLOW")
			}
			changePct := dcGetFloat(r, "CHANGE_PCT")
			if changePct == 0 {
				changePct = dcGetFloat(r, "CHANGE_RATE")
			}
			mainRatio := dcGetFloat(r, "MAIN_NET_INFLOW_RATIO")
			if mainRatio == 0 {
				mainRatio = dcGetFloat(r, "NET_MF_RATIO")
			}
			items = append(items, SectorMoneyFlow{
				Code:      code,
				Name:      name,
				MainNet:   mainNet,
				MainRatio: mainRatio,
				ChangePct: changePct,
			})
		}
		if len(items) > 0 {
			return items, nil
		}
	}

	// Fallback: use per-sector daykline API via push2delay
	return d.fetchSectorMoneyFlowByDateFallback(date)
}

// fetchSectorMoneyFlowByDateFallback queries each sector's daykline API to get historical money flow.
func (d *DataCenterFetcher) fetchSectorMoneyFlowByDateFallback(date string) ([]SectorMoneyFlow, error) {
	date = strings.ReplaceAll(date, "-", "")
	// Get sector list first
	mf := NewMarketFetcher()
	sectors, err := mf.FetchSectorMoneyFlow()
	if err != nil || len(sectors) == 0 {
		return nil, fmt.Errorf("fetchSectorMoneyFlowByDateFallback: cannot get sector list: %w", err)
	}

	type result struct {
		idx  int
		item SectorMoneyFlow
		ok   bool
	}

	ch := make(chan result, len(sectors))
	limiter := make(chan struct{}, 10) // max 10 concurrent

	for i, s := range sectors {
		go func(idx int, code, name string) {
			limiter <- struct{}{}
			defer func() { <-limiter }()

			secID := "90." + code
			url := fmt.Sprintf("https://push2delay.eastmoney.com/api/qt/stock/fflow/daykline/get?secid=%s&fields1=f1,f2,f3,f7&fields2=f51,f52,f53,f54,f55,f56,f57,f62&ut=b2884a393a59ad64002292a3e90d46a5&klt=101&lmt=30", secID)
			body, err := d.doGet(url)
			if err != nil {
				ch <- result{idx: idx, ok: false}
				return
			}

			var resp struct {
				Code int `json:"rc"`
				Data *struct {
					KLines []string `json:"klines"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &resp); err != nil || resp.Code != 0 || resp.Data == nil {
				ch <- result{idx: idx, ok: false}
				return
			}

			for _, line := range resp.Data.KLines {
				parts := strings.Split(line, ",")
				if len(parts) < 8 {
					continue
				}
				kDate := strings.ReplaceAll(parts[0], "-", "")
				if kDate != date {
					continue
				}
				ch <- result{
					idx: idx,
					ok:  true,
					item: SectorMoneyFlow{
						Code:      code,
						Name:      name,
						MainNet:   parseFloat(parts[1]),
						MainRatio: parseFloat(parts[6]),
						ChangePct: parseFloat(parts[7]),
					},
				}
				return
			}
			ch <- result{idx: idx, ok: false}
		}(i, s.Code, s.Name)
	}

	out := make([]SectorMoneyFlow, 0, len(sectors))
	for i := 0; i < len(sectors); i++ {
		r := <-ch
		if r.ok {
			out = append(out, r.item)
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no sector money flow data for %s", date)
	}
	return out, nil
}
