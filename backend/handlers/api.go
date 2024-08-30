package handlers

import (
	"encoding/json"
	"log"
	"main/entities"

	"github.com/gofiber/fiber/v2"
)

func GetAllApis(c *fiber.Ctx) error {
	var api []entities.API
	api, err := entities.GetAllAPIs()

	if err != nil {
		return c.SendStatus(404)
	}

	log.Println("aps", api)

	if len(api) == 0 {
		return c.SendStatus(404)
	}

	for _, ap := range api {
		js, _ := json.Marshal(ap)
		log.Println("marsh", string(js))
	}

	return c.Status(200).JSON(&api)
}
