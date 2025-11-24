package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/models"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/utils"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var validate = validator.New()

type RegisterRequest struct {
	FullName       string  `json:"full_name" validate:"required,min=8"`
	Email          string  `json:"email" validate:"required,email"`
	Password       string  `json:"password" validate:"required,min=6"`
	ReferredByCode *string `json:"referred_by_code,omitempty"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func RegisterUser(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	var newUser models.User
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var referrer *models.User
		if req.ReferredByCode != nil && *req.ReferredByCode != "" {
			if err := tx.Where("referral_code = ?", *req.ReferredByCode).First(&referrer).Error; err != nil {
				log.Printf("Invalid referral code used: %s", *req.ReferredByCode)
				referrer = nil 
			}
		}

		uniqueCode, err := utils.GenerateUniqueReferralCode(tx)
		if err != nil {
			return errors.New("failed to generate unique referral code")
		}

		newUser = models.User{
			FullName:       req.FullName,
			Email:          req.Email,
			Password:       string(hashedPassword),
			ReferralCode:   &uniqueCode,
			ReferredByCode: req.ReferredByCode,
		}
		if err := tx.Create(&newUser).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errors.New("email already exists")
			}
			return err
		}

		if referrer != nil {
			referral := models.Referral{
				ReferrerID:     referrer.ID,
				ReferredUserID: newUser.ID,
				Status:         "pending",
			}
			if err := tx.Create(&referral).Error; err != nil {
				return err
			}
		}
		return nil 
	})

	if err != nil {
		if err.Error() == "email already exists" {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already exists"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	go notifications.SendEmail(newUser.FullName, newUser.Email, "Welcome!", "<h1>Welcome!</h1><p>Thank you for registering.</p>")

	response := UserResponse{
		ID:        newUser.ID.String(),
		FullName:  newUser.FullName,
		Email:     newUser.Email,
		Role:      newUser.Role,
		CreatedAt: newUser.CreatedAt,
	}
	return c.Status(fiber.StatusCreated).JSON(response)
}

func LoginUser(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}
	if err := validate.Struct(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var user models.User
	result := database.DB.Where("email = ?", req.Email).First(&user)
	if result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid email or password"})
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid email or password"})
	}

	claims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 72).Unix(), 
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	
	t, err := token.SignedString([]byte(config.Config("JWT_SECRET")))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create token"})
	}

	return c.JSON(fiber.Map{"token": t})
}


func ForgotPassword(c *fiber.Ctx) error {
	type Request struct {
		Email string `json:"email" validate:"required,email"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "If an account with that email exists, a password reset link has been sent."})
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate reset token"})
	}
	token := hex.EncodeToString(tokenBytes)
	
	expiration := time.Now().Add(15 * time.Minute)
	user.ResetPasswordToken = &token
	user.ResetPasswordTokenExpiresAt = &expiration
	
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save reset token"})
	}

	frontendURL := "https://www.classlearning.co.ke"
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, token)
	
	go notifications.SendEmail(
		user.FullName,
		user.Email,
		"Your Password Reset Link",
		fmt.Sprintf("<h1>Password Reset</h1><p>Click the link below to reset your password. This link is valid for 15 minutes.</p><p><a href='%s'>Reset Password</a></p>", resetLink),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "If an account with that email exists, a password reset link has been sent."})
}

func ResetPassword(c *fiber.Ctx) error {
	type Request struct {
		Token       string `json:"token" validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=6"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"}) }
	if err := validate.Struct(req); err != nil { return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()}) }

	var user models.User
	if err := database.DB.Where("reset_password_token = ?", req.Token).First(&user).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid or expired reset token"})
	}
	
	if user.ResetPasswordTokenExpiresAt.Before(time.Now()) {
		user.ResetPasswordToken = nil
		user.ResetPasswordTokenExpiresAt = nil
		database.DB.Save(&user)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid or expired reset token"})
	}
	
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash new password"})
	}
	
	user.Password = string(hashedPassword)
	user.ResetPasswordToken = nil
	user.ResetPasswordTokenExpiresAt = nil
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update password"})
	}
	
	return c.JSON(fiber.Map{"message": "Password has been reset successfully."})
}