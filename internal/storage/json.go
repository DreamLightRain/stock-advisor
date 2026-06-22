package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"stock-advisor/internal/ai"
	"stock-advisor/internal/stock"
	"sync"
	"time"
)

type Store struct {
	mu       sync.RWMutex
	dataDir  string
}

type Position struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Quantity    float64 `json:"quantity"`
	CostPrice   float64 `json:"costPrice"`
	TargetPrice float64 `json:"targetPrice"`
	StopLoss    float64 `json:"stopLoss"`
	Notes       string  `json:"notes"`
}

type ModelUsage struct {
	Provider      string `json:"provider"`
	ModelName     string `json:"modelName"`
	Endpoint      string `json:"endpoint"`
	APIKey        string `json:"apiKey"`
	Status        string `json:"status"` // "available", "error", "unknown"
	LastTest      string `json:"lastTest"`
	InputTokens   int64  `json:"inputTokens"`
	OutputTokens  int64  `json:"outputTokens"`
	TotalRequests int    `json:"totalRequests"`
}

type AppData struct {
	Stocks             []stock.SelfSelectStock `json:"stocks"`
	Groups             []stock.StockGroup      `json:"groups"`
	Config             ai.Config               `json:"config"`
	RefreshInterval    int                     `json:"refreshInterval"`
	Positions          []Position              `json:"positions"`
	ModelUsages        []ModelUsage            `json:"modelUsages"`
	DataSource         string                  `json:"dataSource"`
	RealTimePriority   []string                `json:"realTimePriority,omitempty"`
	TTSProvider        string                  `json:"ttsProvider"`
	TTSAPIKey          string                  `json:"ttsApiKey"`
}

func NewStore(dataDir string) (*Store, error) {
	s := &Store{dataDir: dataDir}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return s, nil
}

func (s *Store) filePath() string {
	return filepath.Join(s.dataDir, "data.json")
}

func (s *Store) Load() (*AppData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := &AppData{
		Groups: []stock.StockGroup{
			{Name: "自选", Order: 0},
			{Name: "观察中", Order: 1},
			{Name: "已持仓", Order: 2},
		},
		RefreshInterval: 5,
	}

	path := s.filePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return data, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}

	if len(raw) == 0 {
		return data, nil
	}

	if err := json.Unmarshal(raw, data); err != nil {
		// Migration: old data.json used ISO string for addedAt, now int64 (Unix ms)
		if migrated := s.migrateAddedAt(raw); migrated != nil {
			raw = migrated
			data = &AppData{
				Groups: []stock.StockGroup{
					{Name: "自选", Order: 0},
					{Name: "观察中", Order: 1},
					{Name: "已持仓", Order: 2},
				},
				RefreshInterval: 5,
			}
			if err2 := json.Unmarshal(raw, data); err2 != nil {
				return nil, fmt.Errorf("parse data after migration: %w", err2)
			}
		} else {
			return nil, fmt.Errorf("parse data: %w", err)
		}
	}

	if data.Stocks == nil {
		data.Stocks = []stock.SelfSelectStock{}
	}
	if data.Positions == nil {
		data.Positions = []Position{}
	}
	if data.ModelUsages == nil {
		data.ModelUsages = []ModelUsage{}
	}
	if data.Groups == nil {
		data.Groups = []stock.StockGroup{
			{Name: "自选", Order: 0},
			{Name: "观察中", Order: 1},
			{Name: "已持仓", Order: 2},
		}
	}
	if data.RefreshInterval <= 0 {
		data.RefreshInterval = 5
	}

	return data, nil
}

// migrateAddedAt converts old ISO string addedAt to int64 Unix ms.
// Returns migrated raw JSON or nil if no migration was needed/performed.
func (s *Store) migrateAddedAt(raw []byte) []byte {
	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	stocks, ok := root["stocks"].([]interface{})
	if !ok {
		return nil
	}
	changed := false
	for _, st := range stocks {
		stock, ok := st.(map[string]interface{})
		if !ok {
			continue
		}
		addedAt, ok := stock["addedAt"].(string)
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, addedAt)
		if err != nil {
			continue
		}
		stock["addedAt"] = t.UnixMilli()
		changed = true
	}
	if !changed {
		return nil
	}
	migrated, err := json.Marshal(root)
	if err != nil {
		return nil
	}
	// Persist migrated data
	_ = os.WriteFile(s.filePath(), migrated, 0644)
	return migrated
}

func (s *Store) Save(data *AppData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	if err := os.WriteFile(s.filePath(), raw, 0644); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

func (s *Store) AddStock(stock stock.SelfSelectStock) error {
	data, err := s.Load()
	if err != nil {
		return err
	}

	for _, st := range data.Stocks {
		if st.Code == stock.Code {
			return fmt.Errorf("股票 %s (%s) 已在自选中", stock.Name, stock.Code)
		}
	}

	data.Stocks = append(data.Stocks, stock)
	return s.Save(data)
}

func (s *Store) RemoveStock(code string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}

	found := false
	for i, st := range data.Stocks {
		if st.Code == code {
			data.Stocks = append(data.Stocks[:i], data.Stocks[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("股票 %s 不在自选中", code)
	}

	return s.Save(data)
}

func (s *Store) MoveStock(code string, group string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}

	for i, st := range data.Stocks {
		if st.Code == code {
			data.Stocks[i].Group = group
			return s.Save(data)
		}
	}

	return fmt.Errorf("股票 %s 不在自选中", code)
}

func (s *Store) UpdateStockNotes(code string, notes string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}

	for i, st := range data.Stocks {
		if st.Code == code {
			data.Stocks[i].Notes = notes
			return s.Save(data)
		}
	}

	return fmt.Errorf("股票 %s 不在自选中", code)
}

func (s *Store) AddGroup(name string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}

	for _, g := range data.Groups {
		if g.Name == name {
			return fmt.Errorf("分组 %s 已存在", name)
		}
	}

	maxOrder := 0
	for _, g := range data.Groups {
		if g.Order > maxOrder {
			maxOrder = g.Order
		}
	}

	data.Groups = append(data.Groups, stock.StockGroup{
		Name:  name,
		Order: maxOrder + 1,
	})

	return s.Save(data)
}

func (s *Store) RemoveGroup(name string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}

	if len(data.Groups) <= 1 {
		return fmt.Errorf("至少保留一个分组")
	}

	idx := -1
	for i, g := range data.Groups {
		if g.Name == name {
			idx = i
			break
		}
	}

	if idx < 0 {
		return fmt.Errorf("分组 %s 不存在", name)
	}

	defaultGroup := data.Groups[0].Name

	for i, st := range data.Stocks {
		if st.Group == name {
			data.Stocks[i].Group = defaultGroup
		}
	}

	data.Groups = append(data.Groups[:idx], data.Groups[idx+1:]...)

	return s.Save(data)
}

func (s *Store) RenameGroup(oldName, newName string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}

	found := false
	for i, g := range data.Groups {
		if g.Name == oldName {
			data.Groups[i].Name = newName
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("分组 %s 不存在", oldName)
	}

	for i, st := range data.Stocks {
		if st.Group == oldName {
			data.Stocks[i].Group = newName
		}
	}

	return s.Save(data)
}

func (s *Store) SaveConfig(config ai.Config) error {
	data, err := s.Load()
	if err != nil {
		return err
	}

	data.Config = config
	return s.Save(data)
}

func (s *Store) SaveRefreshInterval(interval int) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	if interval < 0 {
		interval = 0
	}
	if interval > 300 {
		interval = 300
	}
	data.RefreshInterval = interval
	return s.Save(data)
}

func (s *Store) SaveDataSource(source string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	data.DataSource = source
	return s.Save(data)
}

func (s *Store) GetTTS() (provider, apiKey string) {
	data, err := s.Load()
	if err != nil {
		return "browser", ""
	}
	return data.TTSProvider, data.TTSAPIKey
}

func (s *Store) SaveTTS(provider, apiKey string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	data.TTSProvider = provider
	data.TTSAPIKey = apiKey
	return s.Save(data)
}

func (s *Store) SaveRealTimePriority(priority []string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	data.RealTimePriority = priority
	return s.Save(data)
}

func (s *Store) GetPosition(code string) *Position {
	data, err := s.Load()
	if err != nil {
		return nil
	}
	for _, p := range data.Positions {
		if p.Code == code {
			return &p
		}
	}
	return nil
}

func (s *Store) GetAllPositions() []Position {
	data, err := s.Load()
	if err != nil {
		return nil
	}
	return data.Positions
}

func (s *Store) SavePosition(pos Position) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	for i, p := range data.Positions {
		if p.Code == pos.Code {
			data.Positions[i] = pos
			return s.Save(data)
		}
	}
	data.Positions = append(data.Positions, pos)
	return s.Save(data)
}

func (s *Store) DeletePosition(code string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	for i, p := range data.Positions {
		if p.Code == code {
			data.Positions = append(data.Positions[:i], data.Positions[i+1:]...)
			return s.Save(data)
		}
	}
	return nil
}

func (s *Store) SaveModelUsage(usage ModelUsage) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	for i, u := range data.ModelUsages {
		if u.Provider == usage.Provider && u.ModelName == usage.ModelName {
			data.ModelUsages[i] = usage
			return s.Save(data)
		}
	}
	data.ModelUsages = append(data.ModelUsages, usage)
	return s.Save(data)
}

func (s *Store) GetModelUsages() []ModelUsage {
	data, err := s.Load()
	if err != nil {
		return nil
	}
	return data.ModelUsages
}

func (s *Store) DeleteModelUsage(provider, modelName string) error {
	data, err := s.Load()
	if err != nil {
		return err
	}
	for i, u := range data.ModelUsages {
		if u.Provider == provider && u.ModelName == modelName {
			data.ModelUsages = append(data.ModelUsages[:i], data.ModelUsages[i+1:]...)
			return s.Save(data)
		}
	}
	return nil
}
