// file: handlers/upload_handler.go
package handlers

import (
	"net/url"
	"strconv"
	"time"

	config "github.com/anjiri1684/language_tutor/configs"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/gofiber/fiber/v2"
)

// GenerateUploadSignature creates a secure signature for a frontend upload.
func GenerateUploadSignature(c *fiber.Ctx) error {
	cloudinaryURL := config.Config("CLOUDINARY_URL")
	cld, err := cloudinary.NewFromURL(cloudinaryURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to initialize Cloudinary"})
	}

	parsedURL, err := url.Parse(cloudinaryURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse Cloudinary URL"})
	}
	secret, _ := parsedURL.User.Password()

	paramsToSign, err := api.StructToParams(uploader.UploadParams{
		Folder: "language_tutor_profiles",
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to prepare signature params"})
	}

	timestamp := time.Now().Unix()
	paramsToSign.Set("timestamp", strconv.FormatInt(timestamp, 10))

	signature, err := api.SignParameters(paramsToSign, secret)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to sign upload params"})
	}

	apiKey := cld.Config.Cloud.APIKey

	return c.JSON(fiber.Map{
		"signature": signature,
		"timestamp": timestamp,
		"api_key":   apiKey,
		"folder":    "language_tutor_profiles",
	})
}
