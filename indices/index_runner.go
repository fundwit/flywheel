package indices

import (
	"flywheel/domain"
	"flywheel/persistence"

	cron "github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

func StartCron() {
	crontab := cron.New(cron.WithSeconds())
	crontab.AddFunc("0 0 23 * * ?", indicesFullSync)
	crontab.Start()
}

func indicesFullSync() {
	page := 1
	pageSize := 500

	db := persistence.ActiveDataSourceManager.GormDB()

	for {
		works := make([]domain.Work, pageSize)
		if err := db.Find(&works).Order("ID ASC").Offset((page - 1) * pageSize).Limit(pageSize).Error; err != nil {
			logrus.Errorf("fully index: page = %d, pageSize = %d, err = %v", page, pageSize, err)
			break
		}

		if len(works) == 0 {
			logrus.Infof("fully index: there are no more work to index")
			break
		}

		IndexWorks(works)
		page++
	}
}
