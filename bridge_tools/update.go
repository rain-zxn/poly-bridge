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
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"poly-bridge/bridge_tools/conf"
	"poly-bridge/crosschaindao"
	"poly-bridge/models"
	"strings"
)

func startUpdate(cfg *conf.UpdateConfig) {
	dao := crosschaindao.NewCrossChainDao(cfg.Server, cfg.DBConfig)
	if dao == nil {
		panic("server is invalid")
	}
	//
	for _, tokenBasic := range cfg.TokenBasics {
		for _, token := range tokenBasic.Tokens {
			token.Hash = strings.ToLower(token.Hash)
		}
	}

	dao.AddTokens(cfg.TokenBasics, cfg.TokenMaps)
	dao.AddChains(cfg.Chains, cfg.ChainFees)
	dao.RemoveTokenMaps(cfg.RemoveTokenMaps)
}

func startUpdate1(cfg *conf.UpdateConfig) {
	Logger := logger.Default
	dbCfg := cfg.DBConfig
	if dbCfg.Debug == true {
		Logger = Logger.LogMode(logger.Info)
	}
	db, err := gorm.Open(mysql.Open(dbCfg.User+":"+dbCfg.Password+"@tcp("+dbCfg.URL+")/"+
		dbCfg.Scheme+"?charset=utf8"), &gorm.Config{Logger: Logger})
	if err != nil {
		panic(err)
	}
	err = db.AutoMigrate(&models.Chain{}, &models.WrapperTransaction{}, &models.ChainFee{}, &models.TokenBasic{}, &models.Token{}, &models.PriceMarket{},
		&models.TokenMap{}, &models.SrcTransaction{}, &models.SrcTransfer{}, &models.PolyTransaction{}, &models.DstTransaction{}, &models.DstTransfer{})
	if err != nil {
		panic(err)
	}
	db.Model(&models.TokenMap{}).Where("1 = 1").Update("property", 1)
	db.Model(&models.Token{}).Where("1 = 1").Update("property", 1)
}