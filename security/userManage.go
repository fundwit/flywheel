package security

import (
	"flywheel/bizerror"
	"flywheel/persistence"
	"github.com/jinzhu/gorm"
)

func UpdateBasicAuthSecret(u *BasicAuthUpdating, sec *Context) error {
	user := User{}
	if err := persistence.ActiveDataSourceManager.GormDB().Model(&User{}).Where(&User{ID: sec.Identity.ID, Secret: HashSha256(u.OriginalSecret)}).Scan(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return bizerror.ErrInvalidPassword
		} else {
			return err
		}
	}

	if err := persistence.ActiveDataSourceManager.GormDB().Model(&User{}).Where(&User{ID: sec.Identity.ID, Secret: HashSha256(u.OriginalSecret)}).
		Update(&User{Secret: HashSha256(u.NewSecret)}).Error; err != nil {
		return err
	}

	return nil
}

func QueryUsers(sec *Context) (*[]UserInfo, error) {
	var users []UserInfo
	if err := persistence.ActiveDataSourceManager.GormDB().Model(&User{}).Scan(&users).Error; err != nil {
		return nil, err
	}
	return &users, nil
}
