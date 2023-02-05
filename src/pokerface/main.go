package main

import (
	"crypto/tls"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/sandrolain/go-pokerface/src/pokerface/forward"
	"github.com/valyala/fasthttp"
)

func main() {

	list := []forward.Forward{
		{
			Method: "GET",
			Path:   "/one",
			Rewrite: forward.Rewrite{
				"/one": "/",
			},
			Destination: "http://localhost:4444",
			Headers: forward.Params{
				"X-Pokerface": "yes",
			},
			Query: forward.Params{
				"Pippo": "pluto",
			},
		},
		{
			Path:        "/two",
			Destination: "http://localhost:5555",
		},
	}

	proxy.WithTlsConfig(&tls.Config{
		InsecureSkipVerify: true,
	})

	proxy.WithClient(&fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
	})

	app := fiber.New(fiber.Config{
		Prefork:       true,
		CaseSensitive: true,
		StrictRouting: true,
	})

	for _, f := range list {
		handler := getPathHandler(f)
		if f.Method != "" {
			app.Add(f.Method, f.Path, handler)
		} else {
			app.All(f.Path, handler)
		}
	}

	app.Listen(":80")
}

func getPathHandler(rew forward.Forward) fiber.Handler {
	rulesRegex := map[*regexp.Regexp]string{}

	if len(rew.Rewrite) > 0 {
		// Initialize
		for k, v := range rew.Rewrite {
			k = strings.Replace(k, "*", "(.*)", -1)
			k = k + "$"
			rulesRegex[regexp.MustCompile(k)] = v
		}
	}

	return func(c *fiber.Ctx) (err error) {
		path := c.Path()
		if len(rulesRegex) > 0 {
			for k, v := range rulesRegex {
				replacer := captureTokens(k, path)
				if replacer != nil {
					path = replacer.Replace(v)
					break
				}
			}
		}

		dest := strings.TrimLeft(rew.Destination, "/") + path

		destUrl, err := url.Parse(dest)
		if err != nil {
			return
		}

		destUrl.RawQuery = string(c.Request().URI().QueryString())

		if len(rew.Headers) > 0 {
			reqHeader := &c.Request().Header
			for k, v := range rew.Headers {
				reqHeader.Add(k, v)
			}
		}

		if len(rew.Query) > 0 {
			query := destUrl.Query()
			for k, v := range rew.Query {
				query.Add(k, v)
			}
			destUrl.RawQuery = query.Encode()
		}

		fn := proxy.Forward(destUrl.String())
		return fn(c)
	}
}

// https://github.com/labstack/echo/blob/master/middleware/rewrite.go
func captureTokens(pattern *regexp.Regexp, input string) *strings.Replacer {
	groups := pattern.FindAllStringSubmatch(input, -1)
	if groups == nil {
		return nil
	}
	values := groups[0][1:]
	replace := make([]string, 2*len(values))
	for i, v := range values {
		j := 2 * i
		replace[j] = "$" + strconv.Itoa(i+1)
		replace[j+1] = v
	}
	return strings.NewReplacer(replace...)
}
