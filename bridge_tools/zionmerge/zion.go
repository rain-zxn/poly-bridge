package zionmerge

import (
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"poly-bridge/conf"
)

/* Steps
 * - createTables
 * - migrateBridgeBasicTables
 */

const (
	CREATE_TABLES       = "createTables"
	MIGRATE_BASIC_TABLES = "migrateBasicTables"

)

func ZionSetUp(cfg *conf.Config) {
	dbCfg := cfg.DBConfig
	Logger := logger.Default
	if dbCfg.Debug == true {
		Logger = Logger.LogMode(logger.Info)
	}

	db, err := gorm.Open(mysql.Open(dbCfg.User+":"+dbCfg.Password+"@tcp("+dbCfg.URL+")/"+
		dbCfg.Scheme+"?charset=utf8"), &gorm.Config{Logger: Logger})
	if err != nil {
		panic(err)
	}
	poly, err := gorm.Open(mysql.Open(dbCfg.User+":"+dbCfg.Password+"@tcp("+dbCfg.URL+")/"+
		dbCfg.PolyScheme+"?charset=utf8"), &gorm.Config{Logger: Logger})
	if err != nil {
		panic(err)
	}

	step := os.Getenv("STEP")
	if step == "" {
		panic("Invalid step")
	}

	switch step {
	case CREATE_TABLES:
		createZionTables(db)
	case MIGRATE_BASIC_TABLES:
		migrateBridgeBasicTables(poly, db)
	default:
		logs.Error("Invalid step %s", step)
	}
}

func checkError(err error, msg string) {
	if err != nil {
		panic(fmt.Sprintf("Fatal: %s error %+v", msg, err))
	}
}

func countTables(tableA, tableB string, src, dst *gorm.DB) {
	var a, b int64
	err := src.Table(tableA).Count(&a).Error
	checkError(err, "count table error")
	err = dst.Table(tableA).Count(&b).Error
	checkError(err, "count table error")
	logs.Info("===============  Compare table size %s %d:%d %s ============", tableA, a, b, tableB)
}
