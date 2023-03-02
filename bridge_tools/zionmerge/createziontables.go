package zionmerge

import (
	"fmt"
	"gorm.io/gorm"
	"poly-bridge/models"
)

func createZionTables(db *gorm.DB) {
	db.DisableForeignKeyConstraintWhenMigrating = true
	err := db.Debug().AutoMigrate(
		&models.AirDropInfo{},
		&models.AssetStatistic{},
		&models.ChainFee{},
		&models.ChainStatistic{},
		&models.Chain{},
		&models.DstSwap{},
		&models.DstTransaction{},
		&models.DstTransfer{},
		&models.LockTokenStatistic{},
		&models.NFTProfile{},
		&models.NftUser{},
		//&models.poly_details
		&models.PolyTransaction{},
		&models.PriceMarket{},
		&models.SrcSwap{},
		&models.SrcTransaction{},
		&models.SrcTransfer{},
		&models.TimeStatistic{},
		&models.TokenBasic{},
		&models.TokenMap{},
		&models.TokenPriceAvg{},
		&models.TokenStatistic{},
		&models.Token{},
		//&models.wrapper_details
		&models.WrapperTransaction{},
	)
	fmt.Println(err)
	checkError(err, "Creating tables")
}
