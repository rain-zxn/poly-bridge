package bridgeeffect

import (
	"encoding/json"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io/ioutil"
	"math/big"
	"net/http"
	"poly-bridge/basedef"
	"poly-bridge/common"
	"poly-bridge/conf"
	"poly-bridge/models"
	"poly-bridge/utils/decimal"
	"time"

	"github.com/beego/beego/v2/core/logs"
)

type AssetDetail struct {
	BasicName  string
	TokenAsset []*DstChainAsset
	Difference *big.Int
	Precision  uint64
	Price      int64
	Amount_usd string
	Reason     string
}
type DstChainAsset struct {
	ChainId     uint64
	Hash        string
	TotalSupply *big.Int
	Balance     *big.Int
	Flow        *big.Int
}

func StartCheckAsset(dbCfg *conf.DBConfig, ipCfg *conf.IPPortConfig) error {
	logs.Info("StartCheckAsset,start startCheckAsset")
	Logger := logger.Default
	if dbCfg.Debug == true {
		Logger = Logger.LogMode(logger.Info)
	}
	db, err := gorm.Open(mysql.Open(dbCfg.User+":"+dbCfg.Password+"@tcp("+dbCfg.URL+")/"+
		dbCfg.Scheme+"?charset=utf8"), &gorm.Config{Logger: Logger})
	if err != nil {
		return err
	}
	resAssetDetails := make([]*AssetDetail, 0)
	extraAssetDetails := make([]*AssetDetail, 0)
	tokenBasics := make([]*models.TokenBasic, 0)
	res := db.
		Where("property = ?", 1).
		Preload("Tokens").
		Find(&tokenBasics)
	if res.Error != nil {
		return err
	}
	for _, basic := range tokenBasics {
		assetDetail := new(AssetDetail)
		dstChainAssets := make([]*DstChainAsset, 0)
		totalFlow := big.NewInt(0)
		for _, token := range basic.Tokens {
			if notToken(token) {
				continue
			}
			if token.Property != int64(1) {
				continue
			}
			chainAsset := new(DstChainAsset)
			chainAsset.ChainId = token.ChainId
			chainAsset.Hash = token.Hash
			balance, err := getAndRetryBalance(token.ChainId, token.Hash)
			if err != nil {
				assetDetail.Reason = err.Error()
				logs.Info("CheckAsset chainId: %v, Hash: %v, err:%v", token.ChainId, token.Hash, err)
				balance = big.NewInt(0)
			}
			chainAsset.Balance = balance
			time.Sleep(time.Second)
			totalSupply, err := getAndRetryTotalSupply(token.ChainId, token.Hash)
			if err != nil {
				assetDetail.Reason = err.Error()
				totalSupply = big.NewInt(0)
				logs.Info("CheckAsset chainId: %v, Hash: %v, err:%v ", token.ChainId, token.Hash, err)
			}
			//specialBasic
			totalSupply = specialBasic(token, totalSupply)
			//original asset
			if !inExtraBasic(token.TokenBasicName) && basic.ChainId == token.ChainId {
				totalSupply = big.NewInt(0)
			}
			chainAsset.TotalSupply = totalSupply
			chainAsset.Flow = new(big.Int).Sub(totalSupply, balance)
			totalFlow = new(big.Int).Add(totalFlow, chainAsset.Flow)
			dstChainAssets = append(dstChainAssets, chainAsset)
		}
		assetDetail.Price = basic.Price
		assetDetail.Precision = basic.Precision
		assetDetail.TokenAsset = dstChainAssets
		assetDetail.Difference = totalFlow
		assetDetail.BasicName = basic.Name
		//03 (WBTC,USDT)
		getO3Data(assetDetail, ipCfg)
		if inExtraBasic(assetDetail.BasicName) {
			extraAssetDetails = append(extraAssetDetails, assetDetail)
			continue
		}
		if assetDetail.Difference.Cmp(big.NewInt(0)) == 1 {
			assetDetail.Amount_usd = decimal.NewFromBigInt(assetDetail.Difference, 0).Div(decimal.New(1, int32(assetDetail.Precision))).Mul(decimal.New(assetDetail.Price, -8)).StringFixed(0)
		}

		resAssetDetails = append(resAssetDetails, assetDetail)
	}
	err = sendDing(resAssetDetails, ipCfg.DingIP)
	if err != nil {
		logs.Error("------------sendDingDINg err---------")
	}
	logs.Info("CheckAsset rightdata___")
	for _, assetDetail := range resAssetDetails {
		logs.Info("CheckAsset:", assetDetail.BasicName, assetDetail.Difference, assetDetail.Precision, assetDetail.Price, assetDetail.Amount_usd)
		for _, tokenAsset := range assetDetail.TokenAsset {
			logs.Info("CheckAsset %2v %-30v %-30v %-30v %-30v\n", tokenAsset.ChainId, tokenAsset.Hash, tokenAsset.TotalSupply, tokenAsset.Balance, tokenAsset.Flow)
		}
	}
	logs.Info("CheckAsset wrongdata___")
	for _, assetDetail := range extraAssetDetails {
		logs.Info("CheckAsset:", assetDetail.BasicName, assetDetail.Difference, assetDetail.Precision, assetDetail.Price)
		for _, tokenAsset := range assetDetail.TokenAsset {
			logs.Info("CheckAsset %2v %-30v %-30v %-30v %-30v\n", tokenAsset.ChainId, tokenAsset.Hash, tokenAsset.TotalSupply, tokenAsset.Balance, tokenAsset.Flow)
		}
	}
	return nil
}
func inExtraBasic(name string) bool {
	extraBasics := []string{"BLES", "GOF", "LEV", "mBTM", "MOZ", "O3", "USDT", "STN", "XMPT"}
	for _, basic := range extraBasics {
		if name == basic {
			return true
		}
	}
	return false
}
func specialBasic(token *models.Token, totalSupply *big.Int) *big.Int {
	presion := decimal.New(1, int32(token.Precision)).BigInt()
	if token.TokenBasicName == "YNI" && token.ChainId == basedef.ETHEREUM_CROSSCHAIN_ID {
		return big.NewInt(0)
	}
	if token.TokenBasicName == "YNI" && token.ChainId == basedef.HECO_CROSSCHAIN_ID {
		return new(big.Int).Mul(big.NewInt(1), presion)
	}
	if token.TokenBasicName == "DAO" && token.ChainId == basedef.ETHEREUM_CROSSCHAIN_ID {
		return new(big.Int).Mul(big.NewInt(1000), presion)
	}
	if token.TokenBasicName == "DAO" && token.ChainId == basedef.HECO_CROSSCHAIN_ID {
		return new(big.Int).Mul(big.NewInt(1000), presion)
	}
	if token.TokenBasicName == "COPR" && token.ChainId == basedef.BSC_CROSSCHAIN_ID {
		return new(big.Int).Mul(big.NewInt(274400000), presion)
	}
	if token.TokenBasicName == "COPR" && token.ChainId == basedef.HECO_CROSSCHAIN_ID {
		return big.NewInt(0)
	}
	if token.TokenBasicName == "DigiCol ERC-721" && token.ChainId == basedef.ETHEREUM_CROSSCHAIN_ID {
		return big.NewInt(0)
	}
	if token.TokenBasicName == "DigiCol ERC-721" && token.ChainId == basedef.HECO_CROSSCHAIN_ID {
		return big.NewInt(0)
	}
	if token.TokenBasicName == "DMOD" && token.ChainId == basedef.ETHEREUM_CROSSCHAIN_ID {
		return big.NewInt(0)
	}
	if token.TokenBasicName == "DMOD" && token.ChainId == basedef.BSC_CROSSCHAIN_ID {
		return new(big.Int).Mul(big.NewInt(15000000), presion)
	}
	if token.TokenBasicName == "SIL" && token.ChainId == basedef.ETHEREUM_CROSSCHAIN_ID {
		x, _ := new(big.Int).SetString("1487520675265330391631", 10)
		return x
	}
	if token.TokenBasicName == "SIL" && token.ChainId == basedef.BSC_CROSSCHAIN_ID {
		return new(big.Int).Mul(big.NewInt(5001), presion)
	}
	if token.TokenBasicName == "DOGK" && token.ChainId == basedef.BSC_CROSSCHAIN_ID {
		return big.NewInt(0)
	}
	if token.TokenBasicName == "DOGK" && token.ChainId == basedef.HECO_CROSSCHAIN_ID {
		x, _ := new(big.Int).SetString("285000000000", 10)
		return new(big.Int).Mul(x, presion)
	}
	if token.TokenBasicName == "SXC" && token.ChainId == basedef.OK_CROSSCHAIN_ID {
		return big.NewInt(0)
	}
	if token.TokenBasicName == "SXC" && token.ChainId == basedef.MATIC_CROSSCHAIN_ID {
		return big.NewInt(0)
	}
	if token.TokenBasicName == "OOE" && token.ChainId == basedef.MATIC_CROSSCHAIN_ID {
		return big.NewInt(0)
	}

	return totalSupply
}
func notToken(token *models.Token) bool {
	if token.TokenBasicName == "USDT" && token.Precision != 6 {
		return true
	}
	return false
}
func getO3Data(assetDetail *AssetDetail, ipCfg *conf.IPPortConfig) {
	switch assetDetail.BasicName {
	case "WBTC":
		chainAsset := new(DstChainAsset)
		chainAsset.ChainId = basedef.O3_CROSSCHAIN_ID
		response, err := http.Get(ipCfg.WBTCIP)
		defer response.Body.Close()
		if err != nil || response.StatusCode != 200 {
			logs.Error("Get o3 WBTC err:", err)
			return
		}
		body, _ := ioutil.ReadAll(response.Body)
		o3WBTC := struct {
			Balance *big.Int
		}{}
		json.Unmarshal(body, &o3WBTC)
		chainAsset.ChainId = basedef.O3_CROSSCHAIN_ID
		chainAsset.Flow = o3WBTC.Balance
		assetDetail.TokenAsset = append(assetDetail.TokenAsset, chainAsset)
		assetDetail.Difference.Add(assetDetail.Difference, chainAsset.Flow)
	case "USDT":
		chainAsset := new(DstChainAsset)
		chainAsset.ChainId = basedef.O3_CROSSCHAIN_ID
		response, err := http.Get(ipCfg.USDTIP)
		defer response.Body.Close()
		if err != nil || response.StatusCode != 200 {
			logs.Error("Get o3 USDT err:", err)
			return
		}
		body, _ := ioutil.ReadAll(response.Body)
		o3USDT := struct {
			Balance *big.Int
		}{}
		json.Unmarshal(body, &o3USDT)
		chainAsset.ChainId = basedef.O3_CROSSCHAIN_ID
		chainAsset.Balance = o3USDT.Balance
		assetDetail.TokenAsset = append(assetDetail.TokenAsset, chainAsset)
	}
}

func getAndRetryBalance(chainId uint64, hash string) (*big.Int, error) {
	balance, err := common.GetBalance(chainId, hash)
	if err != nil {
		for i := 0; i < 2; i++ {
			time.Sleep(time.Second)
			balance, err = common.GetBalance(chainId, hash)
			if err == nil {
				break
			}
		}
	}
	return balance, err
}

func getAndRetryTotalSupply(chainId uint64, hash string) (*big.Int, error) {
	totalSupply, err := common.GetTotalSupply(chainId, hash)
	if err != nil {
		for i := 0; i < 2; i++ {
			time.Sleep(time.Second)
			totalSupply, err = common.GetTotalSupply(chainId, hash)
			if err == nil {
				break
			}
		}
	}
	return totalSupply, err
}

func sendDing(assetDetails []*AssetDetail, dingUrl string) error {
	ss := "[poly_NB]\n"
	flag := false
	for _, assetDetail := range assetDetails {
		if assetDetail.Reason == "all node is not working" {
			continue
		}
		if assetDetail.Difference.Cmp(big.NewInt(0)) == 1 {
			usd, _ := decimal.NewFromString(assetDetail.Amount_usd)
			if usd.Cmp(decimal.NewFromInt32(10000)) == 1 {
				flag = true
				ss += fmt.Sprintf("【%v】totalflow:%v $%v\n", assetDetail.BasicName, decimal.NewFromBigInt(assetDetail.Difference, 0).Div(decimal.New(1, int32(assetDetail.Precision))).StringFixed(2), assetDetail.Amount_usd)
				for _, x := range assetDetail.TokenAsset {
					ss += "ChainId: " + fmt.Sprintf("%v", x.ChainId) + "\n"
					ss += "Hash: " + fmt.Sprintf("%v", x.Hash) + "\n"
					logs.Info("x.TotalSupply:", x.TotalSupply)
					ss += "TotalSupply: " + decimal.NewFromBigInt(x.TotalSupply, 0).Div(decimal.New(1, int32(assetDetail.Precision))).StringFixed(2) + " "
					ss += "Balance: " + decimal.NewFromBigInt(x.Balance, 0).Div(decimal.New(1, int32(assetDetail.Precision))).StringFixed(2) + " "
					ss += "Flow: " + decimal.NewFromBigInt(x.Flow, 0).Div(decimal.New(1, int32(assetDetail.Precision))).StringFixed(2) + "\n"
				}
			}
		}
	}
	if flag {
		err := common.PostDingtext(ss, dingUrl)
		return err
	}
	return nil
}
