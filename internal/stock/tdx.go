package stock

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/injoyai/tdx"
)

type TdxFetcher struct {
	mu     sync.Mutex
	client *tdx.Client
}

func NewTdxFetcher() *TdxFetcher {
	return &TdxFetcher{}
}

func (t *TdxFetcher) Name() string {
	return "通达信(TDX)"
}

func (t *TdxFetcher) ensureConn() error {
	if t.client != nil {
		return nil
	}
	c, err := tdx.DialDefault(tdx.WithRedial(), tdx.WithLevel(tdx.LevelError))
	if err != nil {
		return fmt.Errorf("tdx connect failed: %w", err)
	}
	t.client = c
	return nil
}

func (t *TdxFetcher) FetchRealTime(codes []string) (map[string]*RealTimeData, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("empty codes")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.ensureConn(); err != nil {
		return nil, err
	}

	// Convert codes to TDX format (strip exchange prefix, TDX auto-adds)
	tdxCodes := make([]string, len(codes))
	for i, c := range codes {
		tdxCodes[i] = strings.TrimLeft(c, "shszbj")
	}

	quotes, err := t.client.GetQuote(tdxCodes...)
	if err != nil {
		// Connection may be stale, retry once
		t.client.Close()
		t.client = nil
		if err2 := t.ensureConn(); err2 != nil {
			return nil, fmt.Errorf("tdx reconnect failed: %w", err2)
		}
		quotes, err = t.client.GetQuote(tdxCodes...)
		if err != nil {
			return nil, fmt.Errorf("tdx GetQuote failed: %w", err)
		}
	}

	result := make(map[string]*RealTimeData, len(quotes))
	for _, q := range quotes {
		if q == nil || q.Kline == nil {
			continue
		}
		code := q.Exchange.String() + q.Code
		if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "bj") {
			// keep as is
		} else {
			// find original full code from input
			for _, orig := range codes {
				if strings.HasSuffix(orig, q.Code) {
					code = orig
					break
				}
			}
		}

		last := q.Kline.Last.Float64()
		close := q.Kline.Close.Float64()
		var changePct float64
		if last > 0 {
			changePct = (close - last) / last * 100
		}

		// Map buy levels: BuyLevel[0] might be 买5, BuyLevel[4] might be 买1
		bidPrice := q.BuyLevel[0].Price.Float64()
		askPrice := q.SellLevel[0].Price.Float64()
		// try to get the nearest level (买1 / 卖1)
		for i := 4; i >= 0; i-- {
			if q.BuyLevel[i].Price > 0 && q.BuyLevel[i].Number > 0 {
				bidPrice = q.BuyLevel[i].Price.Float64()
				break
			}
		}
		for i := 0; i < 5; i++ {
			if q.SellLevel[i].Price > 0 && q.SellLevel[i].Number > 0 {
				askPrice = q.SellLevel[i].Price.Float64()
				break
			}
		}

		// Convert 手 to 股
		volume := q.Kline.Volume * 100

		result[code] = &RealTimeData{
			Code:          code,
			Open:          q.Kline.Open.Float64(),
			PrevClose:     last,
			Price:         close,
			High:          q.Kline.High.Float64(),
			Low:           q.Kline.Low.Float64(),
			Volume:        volume,
			Amount:        q.Kline.Amount.Float64(),
			Bid1:          bidPrice,
			Ask1:          askPrice,
			ChangePercent: changePct,
			ChangeAmount:  close - last,
			UpdateTime:    time.Now().Format("15:04:05"),
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("tdx returned no data")
	}
	return result, nil
}

func (t *TdxFetcher) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.client != nil {
		t.client.Close()
		t.client = nil
	}
}
