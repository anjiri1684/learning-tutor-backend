package utils

import (
	"math/rand"
	"time"

	"github.com/anjiri1684/language_tutor/models"
	"gorm.io/gorm"
)

const referralCodeLength = 8
const letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateUniqueReferralCode(tx *gorm.DB) (string, error) {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		b := make([]byte, referralCodeLength)
		for i := range b {
			b[i] = letterBytes[seededRand.Intn(len(letterBytes))]
		}
		code := string(b)

		var user models.User
		err := tx.Where("referral_code = ?", code).First(&user).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return code, nil
			}
			return "", err
		}
	}
}