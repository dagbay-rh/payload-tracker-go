package main

import (
	"github.com/redhatinsights/payload-tracker-go/internal/config"
	"github.com/redhatinsights/payload-tracker-go/internal/db"
	"github.com/redhatinsights/payload-tracker-go/internal/logging"
	models "github.com/redhatinsights/payload-tracker-go/internal/models/db"
)

func main() {
	logging.InitLogger()

	cfg := config.Get()

	db.DbConnect(cfg)

	db.DB.AutoMigrate(
		&models.Services{},
		&models.Sources{},
		&models.Statuses{},
		&models.PayloadStatuses{},
		&models.Payloads{},
	)

	db.DB.Exec("ALTER SEQUENCE payloads_id_seq AS bigint")

	logging.Log.Info("DB Migration Complete")
}
