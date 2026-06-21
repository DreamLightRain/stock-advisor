package analysis

import (
	"fmt"
	"math"
	"stock-advisor/internal/stock"
	"strings"
)

type IndicatorResult struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Signal string  `json:"signal"` // buy/sell/neutral
}

type TechnicalReport struct {
	StockName    string             `json:"stockName"`
	StockCode    string             `json:"stockCode"`
	Indicators   []IndicatorResult  `json:"indicators"`
	Summary      string             `json:"summary"`
	Support      float64            `json:"support"`
	Resistance   float64            `json:"resistance"`
	Suggestion   string             `json:"suggestion"`
	Confidence   int                `json:"confidence"`
}

type Analyzer struct{}

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Analyze(kLines []stock.KLine, realtime *stock.RealTimeData) *TechnicalReport {
	if len(kLines) == 0 {
		return nil
	}

	prices := extractPrices(kLines)
	volumes := extractVolumes(kLines)

	report := &TechnicalReport{
		Indicators: make([]IndicatorResult, 0),
	}

	ma5 := calcMA(prices, 5)
	ma10 := calcMA(prices, 10)
	ma20 := calcMA(prices, 20)

	lastPrice := prices[len(prices)-1]
	currentMA5 := ma5[len(ma5)-1]
	currentMA10 := ma10[len(ma10)-1]
	currentMA20 := ma20[len(ma20)-1]

	report.Indicators = append(report.Indicators,
		IndicatorResult{Name: "MA5", Value: round2(currentMA5), Signal: maSignal(lastPrice, currentMA5)},
		IndicatorResult{Name: "MA10", Value: round2(currentMA10), Signal: maSignal(lastPrice, currentMA10)},
		IndicatorResult{Name: "MA20", Value: round2(currentMA20), Signal: maSignal(lastPrice, currentMA20)},
	)

	// RSI
	rsi6 := calcRSI(prices, 6)
	rsi14 := calcRSI(prices, 14)
	report.Indicators = append(report.Indicators,
		IndicatorResult{Name: "RSI(6)", Value: round2(rsi6), Signal: rsiSignal(rsi6)},
		IndicatorResult{Name: "RSI(14)", Value: round2(rsi14), Signal: rsiSignal(rsi14)},
	)

	// MACD
	macdLine, signalLine, histogram := calcMACD(prices)
	macdV := macdLine[len(macdLine)-1]
	signalV := signalLine[len(signalLine)-1]
	histV := histogram[len(histogram)-1]
	macdSignal := "neutral"
	if macdV > signalV && histV > 0 {
		macdSignal = "buy"
	} else if macdV < signalV && histV < 0 {
		macdSignal = "sell"
	}

	report.Indicators = append(report.Indicators,
		IndicatorResult{Name: "MACD", Value: round2(macdV), Signal: macdSignal},
		IndicatorResult{Name: "MACD Signal", Value: round2(signalV), Signal: macdSignal},
		IndicatorResult{Name: "MACD Histogram", Value: round2(histV), Signal: macdSignal},
	)

	if len(volumes) >= 5 {
		avgVolume := avg(volumes[len(volumes)-5:])
		currentVol := float64(volumes[len(volumes)-1])
		volSignal := "neutral"
		if currentVol > avgVolume*1.5 && lastPrice > prices[len(prices)-2] {
			volSignal = "buy"
		} else if currentVol > avgVolume*1.5 && lastPrice < prices[len(prices)-2] {
			volSignal = "sell"
		}
		report.Indicators = append(report.Indicators,
			IndicatorResult{Name: "Volume Ratio", Value: round2(currentVol / avgVolume), Signal: volSignal},
		)
	}

	// KD
	k, d := calcKD(prices)
	kdSignal := rsiSignal(k - d)
	report.Indicators = append(report.Indicators,
		IndicatorResult{Name: "K", Value: round2(k), Signal: kdSignal},
		IndicatorResult{Name: "D", Value: round2(d), Signal: kdSignal},
	)

	// Support & Resistance
	support, resistance := calcSupportResistance(prices)
	report.Support = round2(support)
	report.Resistance = round2(resistance)

	// MA crossing check
	if len(ma5) >= 2 && len(ma10) >= 2 {
		if ma5[len(ma5)-1] > ma10[len(ma10)-1] && ma5[len(ma5)-2] <= ma10[len(ma10)-2] {
			report.Indicators = append(report.Indicators, IndicatorResult{Name: "MA金叉", Value: 0, Signal: "buy"})
		} else if ma5[len(ma5)-1] < ma10[len(ma10)-1] && ma5[len(ma5)-2] >= ma10[len(ma10)-2] {
			report.Indicators = append(report.Indicators, IndicatorResult{Name: "MA死叉", Value: 0, Signal: "sell"})
		}
	}

	// Generate suggestion
	buyCount := 0
	sellCount := 0
	for _, ind := range report.Indicators {
		if ind.Signal == "buy" {
			buyCount++
		} else if ind.Signal == "sell" {
			sellCount++
		}
	}

	totalSignals := buyCount + sellCount
	if totalSignals > 0 {
		buyRatio := float64(buyCount) / float64(totalSignals)
		if buyRatio >= 0.6 {
			report.Suggestion = "买入"
			report.Confidence = int(buyRatio * 100)
		} else if buyRatio <= 0.4 {
			report.Suggestion = "卖出"
			report.Confidence = int((1 - buyRatio) * 100)
		} else {
			report.Suggestion = "持有/观望"
			report.Confidence = 50
		}
	} else {
		report.Suggestion = "持有/观望"
		report.Confidence = 50
	}

	report.Summary = a.generateSummary(report, lastPrice)

	if realtime != nil {
		report.StockName = realtime.Name
		report.StockCode = realtime.Code
	} else if len(kLines) > 0 {
		report.StockName = ""
		report.StockCode = ""
	}

	return report
}

func (a *Analyzer) KLineSummary(kLines []stock.KLine) string {
	if len(kLines) == 0 {
		return "无数据"
	}
	latest := kLines[len(kLines)-1]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("日期:%s 开:%.2f 收:%.2f 高:%.2f 低:%.2f 量:%.0f万手",
		latest.Date, latest.Open, latest.Close, latest.High, latest.Low,
		float64(latest.Volume)/10000))

	if len(kLines) >= 5 {
		changes := make([]float64, 5)
		for i := 0; i < 5; i++ {
			idx := len(kLines) - 1 - i
			if idx > 0 {
				changes[i] = (kLines[idx].Close - kLines[idx-1].Close) / kLines[idx-1].Close * 100
			}
		}
		sb.WriteString(fmt.Sprintf(" 近5日涨跌幅:%.2f%%~%.2f%%", minFloat(changes), maxFloat(changes)))
	}

	return sb.String()
}

func (a *Analyzer) generateSummary(report *TechnicalReport, price float64) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("当前价: %.2f", price))
	parts = append(parts, fmt.Sprintf("支撑位: %.2f", report.Support))
	parts = append(parts, fmt.Sprintf("压力位: %.2f", report.Resistance))

	for _, ind := range report.Indicators {
		switch ind.Name {
		case "RSI(14)":
			if ind.Value > 70 {
				parts = append(parts, "RSI偏高超买区")
			} else if ind.Value < 30 {
				parts = append(parts, "RSI偏低超卖区")
			}
		case "MACD Histogram":
			if ind.Value > 0 {
				parts = append(parts, "MACD多头动能")
			} else {
				parts = append(parts, "MACD空头动能")
			}
		}
	}

	parts = append(parts, fmt.Sprintf("建议: %s (置信度:%d%%)", report.Suggestion, report.Confidence))

	return strings.Join(parts, " | ")
}

func extractPrices(kLines []stock.KLine) []float64 {
	prices := make([]float64, len(kLines))
	for i, k := range kLines {
		prices[i] = k.Close
	}
	return prices
}

func extractVolumes(kLines []stock.KLine) []int64 {
	vols := make([]int64, len(kLines))
	for i, k := range kLines {
		vols[i] = k.Volume
	}
	return vols
}

func calcMA(prices []float64, period int) []float64 {
	if len(prices) < period {
		period = len(prices)
	}
	result := make([]float64, len(prices))
	sum := 0.0
	for i := 0; i < len(prices); i++ {
		sum += prices[i]
		if i >= period {
			sum -= prices[i-period]
		}
		if i >= period-1 {
			result[i] = sum / float64(period)
		} else {
			result[i] = prices[i]
		}
	}
	return result
}

func calcRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50
	}

	gains, losses := 0.0, 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		diff := prices[i] - prices[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func calcMACD(prices []float64) ([]float64, []float64, []float64) {
	ema12 := calcEMA(prices, 12)
	ema26 := calcEMA(prices, 26)

	macdLine := make([]float64, len(prices))
	for i := 0; i < len(prices); i++ {
		macdLine[i] = ema12[i] - ema26[i]
	}

	signalLine := calcEMA(macdLine, 9)

	histogram := make([]float64, len(prices))
	for i := 0; i < len(prices); i++ {
		histogram[i] = macdLine[i] - signalLine[i]
	}

	return macdLine, signalLine, histogram
}

func calcEMA(prices []float64, period int) []float64 {
	ema := make([]float64, len(prices))
	multiplier := 2.0 / float64(period+1)

	// SMA for first value
	sum := 0.0
	for i := 0; i < period && i < len(prices); i++ {
		sum += prices[i]
	}
	if len(prices) > period {
		ema[period-1] = sum / float64(period)
		for i := period; i < len(prices); i++ {
			ema[i] = (prices[i]-ema[i-1])*multiplier + ema[i-1]
		}
		// Fill early values
		for i := 0; i < period-1; i++ {
			ema[i] = prices[i]
		}
	} else {
		for i := 0; i < len(prices); i++ {
			ema[i] = prices[i]
		}
	}

	return ema
}

func calcKD(prices []float64) (float64, float64) {
	if len(prices) < 9 {
		return 50, 50
	}

	window := prices[len(prices)-9:]
	high := window[0]
	low := window[0]
	for _, p := range window {
		if p > high {
			high = p
		}
		if p < low {
			low = p
		}
	}

	current := window[len(window)-1]
	rsv := 50.0
	if high != low {
		rsv = (current - low) / (high - low) * 100
	}

	k := rsv
	d := k

	// Smooth
	k = 2.0/3.0*k + 1.0/3.0*rsv
	d = 2.0/3.0*d + 1.0/3.0*k

	return k, d
}

func calcSupportResistance(prices []float64) (float64, float64) {
	if len(prices) < 20 {
		if len(prices) > 0 {
			return prices[0] * 0.95, prices[0] * 1.05
		}
		return 0, 0
	}

	recent := prices[len(prices)-20:]
	minP := recent[0]
	maxP := recent[0]
	for _, p := range recent {
		if p < minP {
			minP = p
		}
		if p > maxP {
			maxP = p
		}
	}

	return minP, maxP
}

func maSignal(price, ma float64) string {
	if price > ma*1.02 {
		return "buy"
	} else if price < ma*0.98 {
		return "sell"
	}
	return "neutral"
}

func rsiSignal(rsi float64) string {
	if rsi > 70 {
		return "sell"
	} else if rsi < 30 {
		return "buy"
	}
	return "neutral"
}

func avg(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum int64
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func minFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
	}
	return min
}

func maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}
