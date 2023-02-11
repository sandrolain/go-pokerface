package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/sandrolain/go-pokerface/src/cert"
	"github.com/valyala/fasthttp"
)

type Config struct {
	Forwards []Forward `json:"forwards"`
}

type Rewrite map[string]string
type Params map[string]string

type Forward struct {
	Method      string  `json:"method,omitempty"`
	Path        string  `json:"path"`
	Rewrite     Rewrite `json:"rewrite,omitempty"`
	Destination string  `json:"destination"`
	Query       Params  `json:"query,omitempty"`
	Headers     Params  `json:"headers,omitempty"`
}

func main() {

	if len(os.Args) < 2 {
		panic("Config path required")
	}

	var err error
	configPath := os.Args[1]
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		panic(err)
	}

	_, err = os.Stat(configPath)
	if err != nil {
		panic(err)
	}

	configContent, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	var config Config
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		panic(err)
	}

	proxy.WithTlsConfig(&tls.Config{
		InsecureSkipVerify: true,
	})

	proxy.WithClient(&fasthttp.Client{
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
	})

	app := fiber.New(fiber.Config{
		CaseSensitive: true,
		StrictRouting: true,
	})

	for _, f := range config.Forwards {
		handler := getPathHandler(f)

		if f.Method != "" {
			app.Add(f.Method, f.Path, handler)
		} else {
			app.All(f.Path, handler)
		}
	}

	// ln, err := net.Listen("tcp", ":80")
	// if err != nil {
	// 	panic(err)
	// }

	// app.Listener(ln)

	ln, err := net.Listen("tcp", ":443")
	if err != nil {
		panic(err)
	}

	tlsConfig, err := cert.GenerateTlsConfig()
	if err != nil {
		panic(err)
	}

	app.Listener(tls.NewListener(ln, tlsConfig))
}

func getPathHandler(rew Forward) fiber.Handler {
	rulesRegex := map[*regexp.Regexp]string{}

	if len(rew.Rewrite) > 0 {
		// Initialize
		for k, v := range rew.Rewrite {
			k = strings.Replace(k, "*", "(.*)", -1)
			k = k + "$"
			rulesRegex[regexp.MustCompile(k)] = v
		}
	}

	fmt.Printf("rew: %v\n", rew)

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
				reqHeader.Add(k, getParamsValue(c, v))
			}
		}

		if len(rew.Query) > 0 {
			query := destUrl.Query()
			for k, v := range rew.Query {
				query.Add(k, getParamsValue(c, v))
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

func getHeadersMap(h *fasthttp.RequestHeader) *map[string]string {
	headers := map[string]string{}
	h.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})
	return &headers
}

func getParamsValue(c *fiber.Ctx, value string) string {
	r := regexp.MustCompile(`\{(headers|cookies|query)\.([^}]+)\}`)
	matches := r.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) > 0 {
			var newVal string
			key := match[2]
			switch match[1] {
			case "headers":
				newVal = string(c.Request().Header.Peek(key))
			case "cookies":
				newVal = c.Cookies(key)
			case "query":
				newVal = c.Query(key)
			}
			value = strings.Replace(value, match[0], newVal, 1)
		}
	}
	return value
}
