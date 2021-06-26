package security

import (
	"crypto/sha256"
	"encoding/hex"
	"flywheel/bizerror"
	"flywheel/common"
	"flywheel/persistence"

	"github.com/fundwit/go-commons/types"
	"github.com/jinzhu/gorm"
	"github.com/sony/sonyflake"
)

var (
	userIdWorker *sonyflake.Sonyflake
)

func init() {
	userIdWorker = sonyflake.NewSonyflake(sonyflake.Settings{})
}

func HashSha256(raw string) string {
	h := sha256.New()
	h.Write([]byte(raw))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

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

func CreateUser(c *UserCreation, sec *Context) (*UserInfo, error) {
	if !sec.HasRole(SystemAdminPermission.ID) {
		return nil, bizerror.ErrForbidden
	}

	user := User{ID: common.NextId(userIdWorker), Name: c.Name, Nickname: c.Nickname, Secret: HashSha256(c.Secret)}
	if err := persistence.ActiveDataSourceManager.GormDB().Save(&user).Error; err != nil {
		return nil, err
	}
	return &UserInfo{ID: user.ID, Name: user.Name, Nickname: user.Nickname}, nil
}

func UpdateUser(userId types.ID, c *UserUpdation, sec *Context) error {
	if !sec.HasRole(SystemAdminPermission.ID) && userId != sec.Identity.ID {
		return bizerror.ErrForbidden
	}

	return persistence.ActiveDataSourceManager.GormDB().Transaction(func(tx *gorm.DB) error {
		user := User{ID: userId}
		if err := tx.Where(&user).First(&user).Error; err != nil {
			return err
		}
		if err := tx.Model(&user).Update(&User{Nickname: c.Nickname}).Error; err != nil {
			return err
		}
		return nil
	})
}

func QueryAccountNames(ids []types.ID) (map[types.ID]string, error) {
	if len(ids) == 0 {
		return map[types.ID]string{}, nil
	}
	db := persistence.ActiveDataSourceManager.GormDB()
	var records []UserInfo
	if err := db.Model(&User{}).Where("id IN (?)", ids).Scan(&records).Error; err != nil {
		return nil, err
	}
	result := map[types.ID]string{}
	for _, r := range records {
		result[r.ID] = r.DisplayName()
	}
	return result, nil
}
