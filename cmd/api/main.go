package main

import (
	"log"
	"time"

	"github.com/anjiri1684/language_tutor/database"
	"github.com/anjiri1684/language_tutor/jobs"
	"github.com/anjiri1684/language_tutor/notifications"
	"github.com/anjiri1684/language_tutor/payments"
	"github.com/anjiri1684/language_tutor/routes"
	"github.com/anjiri1684/language_tutor/services"
	"github.com/anjiri1684/language_tutor/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/robfig/cron/v3"
)

func main() {
	database.ConnectDB()
	database.Migrate()
	database.SeedAdmin() 
	notifications.InitEmailService()

	go services.FetchRates()
	go payments.GetKcbAccessToken() 
	c := cron.New()
	c.AddFunc("*/5 * * * *", jobs.CheckForUnattendedClasses)
	c.AddFunc("*/5 * * * *", jobs.SendClassReminders) 
	go c.Start()
	log.Println("✅ Cron job for attendance scheduled successfully.")

	app := fiber.New(fiber.Config{
		Prefork:             false,
		AppName:             "Language Tutor",
		CaseSensitive:       true,
		StrictRouting:       true,
		EnablePrintRoutes:   true,
		PassLocalsToViews:   true,
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        15 * time.Second,
		IdleTimeout:         60 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			log.Printf("[ERROR] %v | Path: %s | Method: %s", err, c.Path(), c.Method())
			return c.Status(code).JSON(fiber.Map{
				"status":  "error",
				"code":    code,
				"message": err.Error(),
			})
		},
	})

	

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*", 
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, Sec-WebSocket-Key, Sec-WebSocket-Version",
		AllowMethods:     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		ExposeHeaders:    "Content-Length, Authorization",
		MaxAge:           86400, 
	}))

	app.Use(recover.New()) 
	app.Use(logger.New(logger.Config{
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Africa/Nairobi",
		Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "success",
			"message": "Welcome to Language Tutor API",
		})
	})


    routes.PublicRoutes(app)
    routes.ProfileRoutes(app)
    routes.AuthRoutes(app)
    routes.TeacherRoutes(app)
    routes.AdminRoutes(app)
    routes.BookingRoutes(app)
    routes.PaymentRoutes(app)
    routes.ResourceRoutes(app)
    routes.UploadRoutes(app)
    routes.ExamRoutes(app)         
    routes.MessagingRoutes(app)   
    routes.GamificationRoutes(app)
    routes.BundleRoutes(app)

	go websocket.RunHub()

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	log.Println("✅ Server is running on port 8080")
	err := app.Listen(":8080")
	if err != nil {
		log.Fatalf("🔥 Server failed to start: %v", err)
	}
}