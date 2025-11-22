package database

import (
	"fmt"
	"log"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	var err error
	dsn := config.Config("DATABASE_URL")

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt: false,
		SkipDefaultTransaction: true,
		DisableForeignKeyConstraintWhenMigrating: true,
		DisableNestedTransaction: true,
		Logger: nil,
		NowFunc: nil,
		Dialector: nil,
	})
	if err != nil {
		log.Fatalf("🔥 Failed to connect to database: %v", err)
	}

	fmt.Println("✅ Database connected successfully")
}

func Migrate() {
	err := DB.AutoMigrate(
		&models.User{}, 
		&models.Teacher{}, 
		&models.AvailabilitySlot{},
		&models.Language{},
		&models.TeacherLanguage{},
		&models.Booking{}, 
		&models.Payment{},
		&models.Question{}, 
		&models.MockTest{},  
		&models.TestAttempt{},   
		&models.AttemptAnswer{}, 
		&models.Badge{}, 
		&models.Review{}, 
		&models.Certificate{}, 
		&models.Conversation{}, 
		&models.Message{},
		&models.Bundle{},        
		&models.StudentBundle{},
		&models.Referral{}, 
		&models.PayoutRequest{},
		&models.Resource{},
) 
	if err != nil {
		log.Fatalf("🔥 Failed to migrate database: %v", err)
	}
	fmt.Println("✅ Database migration successful")
}


func SeedAdmin() {
	adminEmail := config.Config("ADMIN_EMAIL")
	adminPassword := config.Config("ADMIN_PASSWORD")

	var count int64
	err := DB.Model(&models.User{}).Where("email = ?", adminEmail).Count(&count).Error
	if err != nil {
		log.Fatalf("🔥 Failed to check for admin user: %v", err)
		return
	}

	if count > 0 {
		log.Println("Admin user already exists.")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("🔥 Failed to hash admin password: %v", err)
		return
	}

	adminUser := models.User{
		FullName: config.Config("ADMIN_FULL_NAME"),
		Email:    adminEmail,
		Password: string(hashedPassword),
		Role:     "admin",
	}

	if err := DB.Create(&adminUser).Error; err != nil {
		log.Fatalf("🔥 Failed to seed admin user: %v", err)
		return
	}

	log.Println("✅ Admin user seeded successfully")
}