package stock

type StockBasic struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Market    string `json:"market"`
	FullCode  string `json:"fullCode"`
	Type      string `json:"type"`
}

type RealTimeData struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Open            float64 `json:"open"`
	PrevClose       float64 `json:"prevClose"`
	Price           float64 `json:"price"`
	High            float64 `json:"high"`
	Low             float64 `json:"low"`
	Volume          int64   `json:"volume"`
	Amount          float64 `json:"amount"`
	Bid1            float64 `json:"bid1"`
	Ask1            float64 `json:"ask1"`
	ChangePercent   float64 `json:"changePercent"`
	ChangeAmount    float64 `json:"changeAmount"`
	UpdateTime      string  `json:"updateTime"`
}

type KLine struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Volume int64   `json:"volume"`
	Amount float64 `json:"amount"`
}

type StockSuggestion struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	Confidence int   `json:"confidence"`
}

type SelfSelectStock struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	AddedAt   int64  `json:"addedAt"`
	Notes     string `json:"notes"`
}

type StockGroup struct {
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type SearchResult struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	FullCode string `json:"fullCode"`
	Market   string `json:"market"`
	Type     string `json:"type"`
	Industry string `json:"industry,omitempty"`
}

type MoneyFlowItem struct {
	Date         string  `json:"date"`
	MainNet      float64 `json:"mainNet"`      // 主力净流入额 (f52)
	SmallNet     float64 `json:"smallNet"`     // 小单净流入额 (f53)
	MediumNet    float64 `json:"mediumNet"`    // 中单净流入额 (f54)
	LargeNet     float64 `json:"largeNet"`     // 大单净流入额 (f55)
	SuperLargeNet float64 `json:"superLargeNet"` // 超大单净流入额 (f56)
	MainRatio    float64 `json:"mainRatio"`    // 主力净流入占比 (f57)
	Close        float64 `json:"close"`        // 收盘价 (f62)
}
