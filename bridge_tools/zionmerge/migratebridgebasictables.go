package zionmerge

import (
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"gorm.io/gorm"
	"poly-bridge/models"
	"time"
)

func migrateBridgeBasicTables(bri, db *gorm.DB) {
	migrateTable(bri, db, "token_basics", &[]*models.TokenBasic{})
	migrateTable(bri, db, "price_markets", &[]*models.PriceMarket{})
	migrateTable(bri, db, "chains", &[]*models.Chain{})
	migrateTable(bri, db, "chain_fees", &[]*models.ChainFee{})
	migrateTable(bri, db, "nft_profiles", &[]*models.NFTProfile{})
	migrateTable(bri, db, "tokens", &[]*models.Token{})
	migrateTable(bri, db, "token_maps", &[]*models.TokenMap{})
}

func migrateTable(src, dst *gorm.DB, table string, model interface{}) {
	logs.Info("--------- Migrating table %s ---------", table)
	limit, offset := 1000, 0
	for {
		err := src.Limit(limit).Offset(offset).
			Find(model).
			Error
		checkError(err, fmt.Sprintf("Loading table limit: %v offset: %v", limit, offset))
		if len(model.([]interface{})) == 0 {
			break
		}
		err = dst.Save(model).Error
		checkError(err, fmt.Sprintf("Saving table limit: %v offset: %v", limit, offset))
		offset += limit
		time.Sleep(time.Second * 1)
	}
	countTables(table, table, src, dst)

}

