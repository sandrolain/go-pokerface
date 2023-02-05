package main

import "github.com/gofiber/fiber/v2"

type Echo struct {
	Protocol string
	Hostname string
	Method   string
	Path     string
	Query    string
	Headers  map[string]string
	Body     string
}

func main() {
	app := fiber.New()

	app.All("/*", func(c *fiber.Ctx) error {

		e := Echo{
			Protocol: c.Protocol(),
			Hostname: c.Hostname(),
			Method:   c.Method(),
			Path:     c.Path(),
			Query:    string(c.Context().QueryArgs().QueryString()),
			Headers:  c.GetReqHeaders(),
			Body:     string(c.Request().Body()),
		}
		c.JSON(e)
		return nil
	})

	app.Listen(":4444")
}
