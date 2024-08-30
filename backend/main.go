package main

import (
	"log"

	"main/config"
	"main/handlers"

	"github.com/gofiber/fiber/v2"
)

func main() {
	app := fiber.New()

	config.Connect()

	app.Get("/apis", handlers.GetAllApis)

	log.Fatal(app.Listen(":6555"))
}
