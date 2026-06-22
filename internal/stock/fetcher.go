package stock

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type StockFetcher interface {
	Name() string
	FetchRealTime(codes []string) (map[string]*RealTimeData, error)
}

type SourceStats struct {
	Name      string `json:"name"`
	Success   int    `json:"success"`
	Failures  int    `json:"failures"`
	LastError string `json:"lastError"`
}

type FetcherManager struct {
	sina        *SinaFetcher
	tencent     *TencentFetcher
	eastMoney   *EastMoneyFetcher
	dataCenter  *DataCenterFetcher
	tdx         *TdxFetcher

	dataSource        string   // "auto", "eastmoney", "tencent", "sina", "datacenter", "tdx"
	realTimePriority  []string // custom priority order for real-time data
	mu                sync.Mutex

	stats    map[string]*SourceStats
	health   map[string]int // consecutive failures per source
}

func NewFetcherManager() *FetcherManager {
	return &FetcherManager{
		sina:       NewSinaFetcher(),
		tencent:    NewTencentFetcher(),
		eastMoney:  NewEastMoneyFetcher(),
		dataCenter: NewDataCenterFetcher(),
		tdx:        NewTdxFetcher(),
		dataSource: "auto",
		stats:      make(map[string]*SourceStats),
		health:     make(map[string]int),
	}
}

func (fm *FetcherManager) SetDataSource(source string) {
	switch source {
	case "auto", "eastmoney", "tencent", "sina", "datacenter", "tdx":
		fm.dataSource = source
	default:
		fm.dataSource = "auto"
	}
}

func (fm *FetcherManager) SetRealTimePriority(priority []string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.realTimePriority = priority
}

func (fm *FetcherManager) GetRealTimePriority() []string {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if len(fm.realTimePriority) == 0 {
		return []string{"sina", "tencent", "tdx"}
	}
	return fm.realTimePriority
}

func (fm *FetcherManager) GetSourceStats() []*SourceStats {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	result := make([]*SourceStats, 0, len(fm.stats))
	for _, s := range fm.stats {
		result = append(result, s)
	}
	return result
}

func (fm *FetcherManager) trackResult(name string, err error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	s, ok := fm.stats[name]
	if !ok {
		s = &SourceStats{Name: name}
		fm.stats[name] = s
	}
	if err != nil {
		s.Failures++
		s.LastError = err.Error()
		fm.health[name]++
		// Reset health of all other sources to give them a chance
		for n := range fm.health {
			if n != name {
				fm.health[n] = 0
			}
		}
	} else {
		s.Success++
		fm.health[name] = 0 // reset on success
	}
}

func (fm *FetcherManager) isHealthy(name string) bool {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	return fm.health[name] < 3 // 3 consecutive failures = unhealthy
}

func (fm *FetcherManager) sourceFetcher(name string) StockFetcher {
	switch name {
	case "sina":
		return fm.sina
	case "tencent":
		return fm.tencent
	case "tdx":
		return fm.tdx
	}
	return nil
}

func (fm *FetcherManager) SearchStocks(keyword string) ([]SearchResult, error) {
	results, err := fm.eastMoney.Search(keyword)
	if err != nil {
		log.Printf("[搜索] EastMoney failed: %v", err)
		return nil, fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no results for %q", keyword)
	}
	log.Printf("[搜索] found %d results for %q", len(results), keyword)
	return results, nil
}

func (fm *FetcherManager) GetRealTime(codes []string) (map[string]*RealTimeData, error) {
	// Non-auto mode: use specific source
	if fm.dataSource != "auto" && fm.dataSource != "" {
		f := fm.sourceFetcher(fm.dataSource)
		if f != nil {
			result, err := f.FetchRealTime(codes)
			fm.trackResult(f.Name(), err)
			if err == nil && len(result) > 0 {
				return result, nil
			}
			return nil, fmt.Errorf("%s failed: %w", f.Name(), err)
		}
	}

	// Auto mode: try sources in priority order
	priority := fm.GetRealTimePriority()
	for _, name := range priority {
		f := fm.sourceFetcher(name)
		if f == nil {
			continue
		}
		if !fm.isHealthy(f.Name()) {
			log.Printf("[%s] skipping: unhealthy (3+ consecutive failures)", f.Name())
			continue
		}
		result, err := f.FetchRealTime(codes)
		fm.trackResult(f.Name(), err)
		if err == nil && len(result) > 0 {
			return result, nil
		}
		if err != nil {
			log.Printf("[%s] failed: %v", f.Name(), err)
		}
	}

	return nil, fmt.Errorf("all fetchers failed for codes: %v", codes)
}

func (fm *FetcherManager) GetKLine(code string, days int) ([]KLine, error) {
	kline, err := fm.eastMoney.FetchKLine(code, days)
	if err == nil && len(kline) > 0 {
		return kline, nil
	}
	if err != nil {
		log.Printf("[%s] FetchKLine failed: %v, falling back to %s", fm.eastMoney.Name(), err, fm.tencent.Name())
	}
	return fm.tencent.FetchKLine(code, days)
}

func (fm *FetcherManager) GetTimeSharing(code string) ([]KLine, error) {
	return fm.eastMoney.FetchTimeSharing(code)
}

func (fm *FetcherManager) GetMoneyFlow(code string, days int) ([]MoneyFlowItem, error) {
	switch fm.dataSource {
	case "datacenter":
		return fm.dataCenter.FetchMoneyFlow(code, days)
	case "eastmoney":
		return fm.eastMoney.FetchMoneyFlow(code, days)
	case "sina":
		return fm.sina.FetchMoneyFlow(code, days)
	case "tencent":
		// Tencent doesn't have money flow API, fall through to auto
	}

	// auto: try push2delay first (original behavior), fall back to datacenter/historical
	items, err := fm.eastMoney.FetchMoneyFlow(code, days)
	if err == nil && len(items) > 0 {
		return items, nil
	}
	if err != nil {
		log.Printf("[东方财富] FetchMoneyFlow %s failed: %v, falling back to datacenter", code, err)
	}
	return fm.dataCenter.FetchMoneyFlow(code, days)
}

func (fm *FetcherManager) GetMarketSummary() (*MarketSummary, error) {
	mf := NewMarketFetcher()
	return mf.GetSummary()
}

func (fm *FetcherManager) GetStockIndustry(code string) (string, error) {
	return fm.eastMoney.GetStockIndustry(code)
}

func (fm *FetcherManager) Close() {
	if fm.tdx != nil {
		fm.tdx.Close()
	}
}

func (fm *FetcherManager) GetSectorMoneyFlow() ([]SectorMoneyFlow, error) {
	mf := NewMarketFetcher()
	return mf.FetchSectorMoneyFlow()
}

// GetSectorMoneyFlowByDate attempts to get historical sector money flow for a specific date.
// Falls back to real-time data if no historical data available.
func (fm *FetcherManager) GetSectorMoneyFlowByDate(date string) ([]SectorMoneyFlow, error) {
	if date == "" || date == time.Now().Format("2006-01-02") {
		// Today: use real-time
		return fm.GetSectorMoneyFlow()
	}
	// Historical date: try datacenter
	data, err := fm.dataCenter.FetchSectorMoneyFlowByDate(date)
	if err != nil {
		return nil, err
	}
	return data, nil
}


