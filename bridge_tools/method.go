/*
 * Copyright (C) 2020 The poly network Authors
 * This file is part of The poly network library.
 *
 * The  poly network  is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The  poly network  is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 * You should have received a copy of the GNU Lesser General Public License
 * along with The poly network .  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"context"
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/beego/beego/v2/core/logs"
	"github.com/devfans/cogroup"
	"github.com/urfave/cli"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"poly-bridge/basedef"
	"poly-bridge/conf"
	"poly-bridge/crosschaindao"
	"poly-bridge/crosschainlisten"
	"poly-bridge/models"
	"poly-bridge/utils/decimal"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	FETCH_BLOCK = "fetch_block"
)

func executeMethod(method string, ctx *cli.Context) {
	configFile := ctx.GlobalString(getFlagName(configPathFlag))
	fmt.Println("configFile", configFile)
	config := conf.NewConfig(configFile)
	if config == nil {
		logs.Error("startServer - read config failed!")
		return
	}

	switch method {
	case FETCH_BLOCK:
		fetchBlock(config)
	case "bingfaSWTH":
		bingfaSWTH(config)
	case "hecojiaoyi":
		hatNftTx(config)
	case "everyBodyNft":
		everyBodyNft(config)
	default:
		fmt.Printf("Available methods: \n %s", strings.Join([]string{FETCH_BLOCK}, "\n"))
	}
}

func retry(f func() error, count int, duration time.Duration) func(context.Context) error {
	return func(context.Context) error {
		i := 0
		for {
			i++
			if i > count && count != 0 {
				return nil
			}
			err := f()
			if err == nil {
				return nil
			}
			time.Sleep(duration)
		}
		return nil
	}
}

func fetchSingleBlock(chainId, height uint64, handle crosschainlisten.ChainHandle, dao crosschaindao.CrossChainDao, save bool) error {
	wrapperTransactions, srcTransactions, polyTransactions, dstTransactions, locks, unlocks, err := handle.HandleNewBlock(height)
	if err != nil {
		logs.Error(fmt.Sprintf("HandleNewBlock %d err: %v", height, err))
		return err
	}
	if save {
		err = dao.UpdateEvents(wrapperTransactions, srcTransactions, polyTransactions, dstTransactions)
		if err != nil {
			return nil
		}
	}
	fmt.Printf(
		"Fetch block events success chain %d height %d wrapper %d src %d poly %d dst %d  locks %d unlocks %d \n",
		chainId, height, len(wrapperTransactions), len(srcTransactions), len(polyTransactions), len(dstTransactions), locks, unlocks,
	)
	for i, tx := range wrapperTransactions {
		fmt.Printf("wrapper %d: %+v\n", i, *tx)
	}
	for i, tx := range srcTransactions {
		fmt.Printf("src %d: %+v srcTransfer:%+v\n", i, *tx, tx.SrcTransfer)
	}
	for i, tx := range polyTransactions {
		fmt.Printf("poly %d: %+v\n", i, *tx)
	}
	for i, tx := range dstTransactions {
		fmt.Printf("dst %d: %+v\n", i, *tx)
	}
	return nil
}

func fetchBlock(config *conf.Config) {
	height, _ := strconv.Atoi(os.Getenv("BR_HEIGHT"))
	chain, _ := strconv.Atoi(os.Getenv("BR_CHAIN"))
	save := os.Getenv("BR_SAVE")
	endheight, _ := strconv.Atoi(os.Getenv("END_HEIGHT"))
	if endheight < height {
		endheight = height
	}
	if height == 0 || chain == 0 {
		panic(fmt.Sprintf("Invalid param chain %d height %d", chain, height))
	}

	dao := crosschaindao.NewCrossChainDao(basedef.SERVER_POLY_BRIDGE, false, config.DBConfig)
	if dao == nil {
		panic("server is not valid")
	}

	var handle crosschainlisten.ChainHandle
	for _, cfg := range config.ChainListenConfig {
		if int(cfg.ChainId) == chain {
			handle = crosschainlisten.NewChainHandle(cfg)
			break
		}
	}

	if handle == nil {
		panic(fmt.Sprintf("chain %d handler is invalid", chain))
	}

	g := cogroup.Start(context.Background(), 4, 8, false)
	for h := height; h <= endheight; h++ {
		block := uint64(h)
		g.Insert(retry(func() error {
			return fetchSingleBlock(uint64(chain), block, handle, dao, save == "true")
		}, 0, 2*time.Second))
	}
	g.Wait()
	/*
		for hei := height; hei <= endheight; {
			ch:=make(chan bool,5)
			for h:=hei;h<hei+5;h++ {
				go func() {
					wrapperTransactions, srcTransactions, polyTransactions, dstTransactions, locks, unlocks, err := handle.HandleNewBlock(uint64(h))
					if err != nil {
						logs.Error(fmt.Sprintf("HandleNewBlock %d err: %v", h, err))
						time.Sleep(time.Millisecond * 500)
						ch<-false
					}
					if save == "true" {
						err = dao.UpdateEvents(wrapperTransactions, srcTransactions, polyTransactions, dstTransactions)
						if err != nil {
							panic(err)
						}
					}
					fmt.Printf(
						"Fetch block events success chain %d height %d wrapper %d src %d poly %d dst %d  locks %d unlocks %d \n",
						chain, h, len(wrapperTransactions), len(srcTransactions), len(polyTransactions), len(dstTransactions), locks, unlocks,
					)
					for i, tx := range wrapperTransactions {
						fmt.Printf("wrapper %d: %+v\n", i, *tx)
					}
					for i, tx := range srcTransactions {
						fmt.Printf("src %d: %+v srcTransfer:%+v\n", i, *tx, tx.SrcTransfer)
					}
					for i, tx := range polyTransactions {
						fmt.Printf("poly %d: %+v\n", i, *tx)
					}
					for i, tx := range dstTransactions {
						fmt.Printf("dst %d: %+v\n", i, *tx)
					}
					ch<-true
				}()
			}
			for k:=0;k<5;k++{
				if <-ch ==false{
					break
				}
			}
			hei+=5
		}
	*/
}

func bingfaSWTH(config *conf.Config) {
	dao := crosschaindao.NewCrossChainDao(basedef.SERVER_POLY_BRIDGE, false, config.DBConfig)
	if dao == nil {
		panic("server is not valid")
	}
	var handle crosschainlisten.ChainHandle
	for _, cfg := range config.ChainListenConfig {
		if cfg.ChainId == basedef.SWITCHEO_CROSSCHAIN_ID {
			handle = crosschainlisten.NewChainHandle(cfg)
			break
		}
	}
	if handle == nil {
		panic(fmt.Sprintf("chain %d handler is invalid", basedef.SWITCHEO_CROSSCHAIN_ID))
	}
	Logger := logger.Default
	dbCfg := config.DBConfig
	if dbCfg.Debug == true {
		Logger = Logger.LogMode(logger.Info)
	}
	db, err := gorm.Open(mysql.Open(dbCfg.User+":"+dbCfg.Password+"@tcp("+dbCfg.URL+")/"+
		dbCfg.Scheme+"?charset=utf8"), &gorm.Config{Logger: Logger})
	srcHeights := make([]int, 0)
	dstHeights := make([]int, 0)
	err = db.Table("src_transactions").
		Select("height").
		Where("chain_id = ?", basedef.SWITCHEO_CROSSCHAIN_ID).
		Find(&srcHeights).Error
	if err != nil {
		panic(fmt.Sprintf("bingfaSWTH db Find(&inHeights) err:%v", err))
	}
	fmt.Println("bingfaSWTH Find(&srcHeights)", srcHeights[:3])
	err = db.Table("dst_transactions").
		Select("height").
		Where("chain_id = ?", basedef.SWITCHEO_CROSSCHAIN_ID).
		Find(&dstHeights).Error
	if err != nil {
		panic(fmt.Sprintf("bingfaSWTH db Find(&dstHeights) err:%v", err))
	}
	fmt.Println("bingfaSWTH Find(&dstHeights)", dstHeights[:3])
	heights := srcHeights
	heights = append(heights, dstHeights...)

	for _, height := range heights {
		wrapperTransactions, srcTransactions, polyTransactions, dstTransactions, _, _, err := handle.HandleNewBlock(uint64(height))
		if err != nil {
			panic(fmt.Sprintf("bingfaSWTH HandleNewBlock %d err: %v", height, err))
		}
		err = dao.UpdateEvents(wrapperTransactions, srcTransactions, polyTransactions, dstTransactions)
		if err != nil {
			panic(fmt.Sprintf("bingfaSWTH bingfaSWTH panic panicHeight:%v,flagerr is:%v", height, err))
		}
		fmt.Printf("bingfaSWTH ing.....nowHeight:%v /n", height)
	}
}
func everyBodyNft(config *conf.Config) {
	type User struct {
		Address     string
		FirstChain  uint64
		Firsttime   string
		TotalAmount *models.BigInt
	}

	type RecordUser struct {
		Address     string `xlsx:"A-Address"`
		FirstChain  string `xlsx:"B-FirstChain"`
		Firsttime   string `xlsx:"C-Firsttime"`
		TotalAmount string `xlsx:"D-TotalAmount"`
	}

	Logger := logger.Default
	dbCfg := config.DBConfig
	if dbCfg.Debug == true {
		Logger = Logger.LogMode(logger.Info)
	}
	db, err := gorm.Open(mysql.Open(dbCfg.User+":"+dbCfg.Password+"@tcp("+dbCfg.URL+")/"+
		dbCfg.Scheme+"?charset=utf8"), &gorm.Config{Logger: Logger})
	if err != nil {
		panic(fmt.Sprintf("database err", err))
	}
	chains := []*models.Chain{}
	err = db.Find(&chains).Error
	if err != nil {
		panic(err)
	}
	models.Init(chains)
	users := make([]*User, 0)
	err = db.Raw("select convert(sum(a.amount*10000/POW(10,b.precision)),decimal(37,0)) as total_amount,a.chain_id as first_chain,a.`from` as address from src_transfers a left join tokens b on a.chain_id =b.chain_id and a.asset=b.hash where a.`from`<> '' and  a.`from` is not null and chain_id <> 0 group by a.chain_id,a.`from`").
		Scan(&users).Error
	if err != nil {
		panic(fmt.Sprint("Find(&recordUsers).Error:", err))
	}
	recordUsers := make([]interface{}, 0)
	for i, user := range users {
		if i >= 5 {
			break
		}
		var tt int64
		err = db.Raw("select time from src_transfers where `from`= ? and chain_id= ? order by time limit 1", user.Address, user.FirstChain).
			First(&tt).Error
		if err != nil {
			logs.Error("First(&tt).Error", err)
		}
		recordUser := new(RecordUser)
		recordUser.Firsttime = time.Unix(tt, 0).Format("2006-01-02")
		recordUser.Address = basedef.Hash2Address(user.FirstChain, user.Address)
		recordUser.FirstChain = models.ChainId2Name(user.FirstChain)
		recordUser.TotalAmount = decimal.NewFromBigInt(&user.TotalAmount.Int, -4).String()
		recordUsers = append(recordUsers, recordUser)
	}
	RefactorWrite(recordUsers, "allUsers_nft.xlsx")
}

func hatNftTx(config *conf.Config) {
	Logger := logger.Default
	dbCfg := config.DBConfig
	if dbCfg.Debug == true {
		Logger = Logger.LogMode(logger.Info)
	}
	db, err := gorm.Open(mysql.Open(dbCfg.User+":"+dbCfg.Password+"@tcp("+dbCfg.URL+")/"+
		dbCfg.Scheme+"?charset=utf8"), &gorm.Config{Logger: Logger})
	if err != nil {
		panic(fmt.Sprintf("database err", err))
	}
	type BuC struct {
		Token   string
		Address string
		Amount  *models.BigInt
	}
	type RecordToken struct {
		Token   string `xlsx:"A-TOKEN"`
		Address string `xlsx:"B-Address"`
		Amount  string `xlsx:"C-Amount"`
	}
	chainId := 7
	tokenHash := map[string]string{"c38072aa3f8e049de541223a9c9772132bb48634": "SHIB",
		"485cdbff08a4f91a16689e73893a11ae8b76af6d": "FEI",
		"4f99d10e16972ff2fe315eee53a95fc5a5870ce3": "BNB"}
	records := make([]interface{}, 0)
	for k, v := range tokenHash {
		buCs := make([]*BuC, 0)
		err := db.Raw("select a.`to` as address, sum(a.amount) as amount from dst_transfers a left join dst_transactions b on a.tx_hash=b.hash  where a.chain_id = ? and a.asset=? and b.time < 1628589600 group by a.`to`;", chainId, k).
			Find(&buCs).Error
		if err != nil {
			logs.Error("Find(&addressAndAmounts),Error", err)
		}
		token := new(models.Token)
		err = db.Where("hash = ? and chain_id = ?", k, chainId).
			Preload("TokenBasic").
			First(token).Error
		if err != nil {
			logs.Error("First(token),Error", err)
		}
		for _, buc := range buCs {
			record := new(RecordToken)
			record.Token = v
			record.Address = buc.Address
			price := decimal.New(token.TokenBasic.Price, -8)
			amount_usd := decimal.NewFromBigInt(&buc.Amount.Int, -18).Mul(price)
			if amount_usd.Cmp(decimal.NewFromInt(0)) == 1 {
				record.Amount = amount_usd.StringFixed(4)
				records = append(records, record)
			}
		}
	}
	RefactorWrite(records, "heco_nft.xlsx")
}
func RefactorWrite(records []interface{}, path string) {
	xlsx := excelize.NewFile()
	index := xlsx.NewSheet("Sheet1")

	for i, t := range records {
		d := reflect.TypeOf(t).Elem()
		for j := 0; j < d.NumField(); j++ {
			// 设置表头
			if i == 0 {
				column := strings.Split(d.Field(j).Tag.Get("xlsx"), "-")[0]
				name := strings.Split(d.Field(j).Tag.Get("xlsx"), "-")[1]
				xlsx.SetCellValue("Sheet1", fmt.Sprintf("%s%d", column, i+1), name)
			}
			// 设置内容
			column := strings.Split(d.Field(j).Tag.Get("xlsx"), "-")[0]
			switch d.Field(j).Type.String() {
			case "string":
				xlsx.SetCellValue("Sheet1", fmt.Sprintf("%s%d", column, i+2), reflect.ValueOf(t).Elem().Field(j).String())
			case "float32":
				xlsx.SetCellValue("Sheet1", fmt.Sprintf("%s%d", column, i+2), reflect.ValueOf(t).Elem().Field(j).Float())
			}
		}
	}
	xlsx.SetActiveSheet(index)
	err := xlsx.SaveAs(path)
	if err != nil {
		fmt.Println(err)
	}
}
