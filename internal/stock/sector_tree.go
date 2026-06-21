package stock

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type SectorLeader struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	ChangePct float64 `json:"changePct"`
}

type SectorTreeNode struct {
	Code      string           `json:"code"`
	Name      string           `json:"name"`
	Level     int              `json:"level"`
	MainNet   float64          `json:"mainNet"`
	ChangePct float64          `json:"changePct"`
	Children  []SectorTreeNode `json:"children,omitempty"`
	Leader    *SectorLeader    `json:"leader,omitempty"`
}

// bk0Keywords maps BK0 code to keywords for matching BK1 children
var bk0Keywords = map[string][]string{
	"BK0427": {"电力", "光伏", "风电", "电池", "电网", "输变电", "配电", "线缆", "电机", "电气", "锂电", "充电", "电源"},
	"BK0428": {"汽车整车", "汽车服务", "摩托车"},
	"BK0433": {"农业", "林业", "牧业", "渔业", "种植", "养殖", "饲料", "种子", "土地"},
	"BK0436": {"纺织", "服装", "鞋", "家纺"},
	"BK0437": {"煤炭", "焦炭"},
	"BK0438": {"食品", "饮料", "白酒", "啤酒", "乳品", "调味", "肉", "烘焙", "食品加工"},
	"BK0440": {"家具", "家居", "造纸", "包装", "印刷", "文娱"},
	"BK0448": {"通信设备", "通信", "5G", "光通信", "移动通信"},
	"BK0450": {"公路", "铁路", "港口", "航运", "航空", "机场", "物流", "快递", "运输"},
	"BK0451": {"房地产", "商业地产", "住宅开发", "园区"},
	"BK0454": {"银行", "证券", "保险", "信托", "金融", "期货"},
	"BK0456": {"家电", "空调", "冰箱", "洗衣机", "厨电", "小家电"},
	"BK0457": {"机械", "机床", "工程机械", "农机", "仪器", "仪表", "工业"},
	"BK0458": {"仪器", "仪表", "传感器", "计量"},
	"BK0459": {"电子", "半导体", "芯片", "集成电路", "元件", "PCB", "LED", "光学", "消费电子"},
	"BK0464": {"石油", "石化", "原油", "天然气", "油气", "炼化"},
	"BK0465": {"化学制药", "医药", "药", "原料药", "制剂"},
	"BK0471": {"化学纤维", "化纤", "氨纶", "涤纶", "粘胶"},
	"BK0473": {"证券", "券商"},
	"BK0474": {"银行"},
	"BK0475": {"建筑装饰", "装修", "园林", "幕墙"},
	"BK0476": {"装修", "建材", "幕墙", "钢结构"},
	"BK0478": {"有色", "金属", "黄金", "铜", "铝", "锌", "铅", "镍", "稀土", "钨", "钼", "钛"},
	"BK0479": {"航运", "港口", "船舶", "海洋"},
	"BK0481": {"汽车零部件", "汽车配件", "轮胎", "车灯", "底盘", "发动机", "变速器"},
	"BK0482": {"医疗", "医院", "医疗器械", "医药商业", "药店", "医疗服务"},
	"BK0484": {"贸易", "进出口", "外贸"},
	"BK0486": {"传媒", "影视", "游戏", "广告", "出版", "广电", "互联网"},
	"BK0538": {"化工", "化学", "化肥", "农药", "涂料", "染料", "橡胶", "塑料", "树脂", "助剂"},
	"BK0539": {"综合"},
	"BK0545": {"通信设备", "线缆", "光缆"},
	"BK0546": {"国防", "军工", "航天", "航空", "舰船", "兵器"},
	"BK0725": {"装修装饰", "园林"},
	"BK0726": {"咨询", "检测", "服务"},
	"BK0727": {"医疗服务", "医院", "诊断", "康复"},
	"BK0728": {"酒店", "餐饮", "旅游", "景点", "度假"},
	"BK0731": {"农产品", "农副", "粮油", "糖", "棉"},
	"BK0732": {"建材", "水泥", "玻璃", "陶瓷", "管材"},
	"BK0734": {"商品", "贸易", "零售", "百货", "超市", "连锁"},
	"BK0735": {"医疗设备", "器械", "耗材"},
	"BK0736": {"通信服务", "电信", "运营"},
	"BK0737": {"电力", "发电", "水电", "火电", "核电", "风电", "新能源电力"},
	"BK0738": {"多元金融", "金控", "租赁", "担保"},
	"BK0739": {"工程机械", "重工", "挖掘机", "起重机"},
	"BK0740": {"环保", "环境", "水务", "固废", "污水处理", "大气", "节能"},
	"BK0910": {"专用设备", "纺织机械", "印刷机械", "医疗设备"},
	"BK0420": {"环境治理", "环保", "生态"},
	"BK0421": {"铁路", "公路", "高速"},
	"BK0422": {"航空", "机场"},
	"BK0424": {"水务", "水处理", "供水"},
}

func (fm *FetcherManager) GetSectorTree() ([]SectorTreeNode, error) {
	mf := NewMarketFetcher()
	allSectors, err := mf.FetchSectorMoneyFlow()
	if err != nil {
		return nil, err
	}

	var bk0Nodes, bk1Nodes []SectorMoneyFlow
	for _, s := range allSectors {
		if strings.HasPrefix(s.Code, "BK0") {
			bk0Nodes = append(bk0Nodes, s)
		} else if strings.HasPrefix(s.Code, "BK1") {
			bk1Nodes = append(bk1Nodes, s)
		}
	}

	used := make(map[string]bool)
	tree := make([]SectorTreeNode, 0, len(bk0Nodes))

	for _, l1 := range bk0Nodes {
		keywords := bk0Keywords[l1.Code]
		if len(keywords) == 0 {
			continue
		}
		children := make([]SectorTreeNode, 0)
		for _, l2 := range bk1Nodes {
			if used[l2.Code] {
				continue
			}
			if matchesBK0(l2.Name, keywords) {
				used[l2.Code] = true
				leader := fm.getSectorLeader(l2.Code)
				children = append(children, SectorTreeNode{
					Code:      l2.Code,
					Name:      l2.Name,
					Level:     2,
					MainNet:   l2.MainNet,
					ChangePct: l2.ChangePct,
					Leader:    leader,
				})
			}
		}
		if len(children) > 0 {
			sort.Slice(children, func(i, j int) bool {
				return children[i].MainNet > children[j].MainNet
			})
			tree = append(tree, SectorTreeNode{
				Code:      l1.Code,
				Name:      l1.Name,
				Level:     1,
				MainNet:   l1.MainNet,
				ChangePct: l1.ChangePct,
				Children:  children,
			})
		}
	}

	// unmatched BK1 → "其他"
	unmatched := make([]SectorTreeNode, 0)
	for _, l2 := range bk1Nodes {
		if !used[l2.Code] {
			leader := fm.getSectorLeader(l2.Code)
			unmatched = append(unmatched, SectorTreeNode{
				Code:      l2.Code,
				Name:      l2.Name,
				Level:     2,
				MainNet:   l2.MainNet,
				ChangePct: l2.ChangePct,
				Leader:    leader,
			})
		}
	}
	if len(unmatched) > 0 {
		sort.Slice(unmatched, func(i, j int) bool {
			return unmatched[i].MainNet > unmatched[j].MainNet
		})
		tree = append(tree, SectorTreeNode{
			Code:      "OTHER",
			Name:      "其他",
			Level:     1,
			Children:  unmatched,
		})
	}

	sort.Slice(tree, func(i, j int) bool {
		return tree[i].MainNet > tree[j].MainNet
	})
	return tree, nil
}

func matchesBK0(name string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

func (fm *FetcherManager) getSectorLeader(bkCode string) *SectorLeader {
	url := fmt.Sprintf("https://push2delay.eastmoney.com/api/qt/clist/get?pn=1&pz=1&np=1&po=0&fltt=2&invt=2&fid=f62&fs=b:%s&fields=f12,f14,f2,f3,f62&ut=b2884a393a59ad64002292a3e90d46a5", bkCode)
	mf := NewMarketFetcher()
	body, err := mf.doGet(url)
	if err != nil {
		return nil
	}
	var result struct {
		Data *struct {
			Diff []struct {
				F12 string  `json:"f12"`
				F14 string  `json:"f14"`
				F2  float64 `json:"f2"`
				F3  float64 `json:"f3"`
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Data == nil || len(result.Data.Diff) == 0 {
		return nil
	}
	d := result.Data.Diff[0]
	return &SectorLeader{
		Code:      d.F12,
		Name:      d.F14,
		Price:     d.F2,
		ChangePct: d.F3,
	}
}
