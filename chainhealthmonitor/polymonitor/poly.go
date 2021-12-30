package polymonitor

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"github.com/polynetwork/poly-go-sdk"
	"math"
	"poly-bridge/basedef"
	"poly-bridge/cacheRedis"
	"poly-bridge/conf"
	"time"
)

type PolyHealthMonitor struct {
	monitorConfig *conf.HealthMonitorConfig
	sdks          map[string]*poly_go_sdk.PolySdk
	nodeHeight    map[string]uint64
	nodeStatus    map[string]string
}

func NewPolyHealthMonitor(monitorConfig *conf.HealthMonitorConfig) *PolyHealthMonitor {
	polyMonitor := &PolyHealthMonitor{}
	polyMonitor.monitorConfig = monitorConfig
	sdks := make(map[string]*poly_go_sdk.PolySdk, 0)
	for _, node := range monitorConfig.ChainNodes.Nodes {
		sdk := poly_go_sdk.NewPolySdk()
		sdk.NewRpcClient().SetAddress(node.Url)
		sdks[node.Url] = sdk
	}
	polyMonitor.sdks = sdks
	return polyMonitor
}

func (p *PolyHealthMonitor) GetChainName() string {
	return p.monitorConfig.ChainName
}

func (p *PolyHealthMonitor) NodeMonitor() error {
	nodeStatuses := make([]basedef.NodeStatus, 0)
	for url, sdk := range p.sdks {
		status := basedef.NodeStatus{
			ChainId:   p.monitorConfig.ChainId,
			ChainName: p.monitorConfig.ChainName,
			Url:       url,
			Time:      time.Now().Format("2006-01-02 15:04:05"),
		}
		height, err := p.GetCurrentHeight(sdk, url)
		if err == nil {
			status.Height = height
			p.nodeHeight[url] = height
			err = p.CheckAbiCall(sdk, url)
		}
		if err != nil {
			p.nodeStatus[url] = err.Error()
		} else {
			p.nodeStatus[url] = "OK"
		}
		status.Status = p.nodeStatus[url]
		nodeStatuses = append(nodeStatuses, status)
	}
	data, _ := json.Marshal(nodeStatuses)
	_, err := cacheRedis.Redis.Set(cacheRedis.NodeStatusPrefix+p.monitorConfig.ChainName, data, time.Hour*24)
	if err != nil {
		logs.Error("set neo node status error: %s", err)
	}
	return err
}

func (p *PolyHealthMonitor) GetCurrentHeight(sdk *poly_go_sdk.PolySdk, url string) (uint64, error) {
	height, err := sdk.GetCurrentBlockHeight()
	if err != nil || height == 0 || height == math.MaxUint32 {
		err := fmt.Errorf("get current block height err: %s", err)
		logs.Error(fmt.Sprintf("poly node: %s, %s ", url, err))
		return 0, err
	}
	logs.Info("poly node: %s, latest height: %d", url, height)
	return uint64(height), nil
}

func (p *PolyHealthMonitor) CheckAbiCall(sdk *poly_go_sdk.PolySdk, url string) error {
	_, err := sdk.GetSmartContractEventByBlock(uint32(p.nodeHeight[url]) - 1)
	if err != nil {
		err := fmt.Errorf("call GetSmartContractEventByBlock err: %s", err)
		logs.Error(fmt.Sprintf("poly node: %s, %s ", url, err))
		return err
	}
	return nil
}