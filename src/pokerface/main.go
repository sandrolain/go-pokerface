package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/francoispqt/gojay"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/r3labs/diff/v3"
	"github.com/sandrolain/go-pokerface/src/cert"
	"github.com/sandrolain/go-pokerface/src/pokerface/shared"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/valyala/fasthttp"
)

type Config struct {
	Port     int       `json:"port"`
	Https    bool      `json:"https"`
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
	WasmFilter  string  `json:"wasmFilter,omitempty"`
	WasmData    []byte
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

	app.Use(logger.New(logger.Config{
		Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
	}))

	for _, f := range config.Forwards {
		handler, err := getPathHandler(f)
		if err != nil {
			panic(err)
		}

		if f.Method != "" {
			app.Add(f.Method, f.Path, handler)
		} else {
			app.All(f.Path, handler)
		}
	}

	addr := fmt.Sprintf(":%v", config.Port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	if config.Https {
		tlsConfig, err := cert.GenerateTlsConfig()
		if err != nil {
			panic(err)
		}

		ln = tls.NewListener(ln, tlsConfig)
	}

	app.Listener(ln)
}

func getPathHandler(rew Forward) (fiber.Handler, error) {
	if rew.WasmFilter != "" {
		wasmBytes, err := ioutil.ReadFile(rew.WasmFilter)
		if err != nil {
			return nil, err
		}
		rew.WasmData = wasmBytes
	}

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

		if len(rew.WasmData) > 0 {
			err = applyWasmFilter(&rew, c)
			if err != nil {
				return
			}
		}

		fn := proxy.Forward(destUrl.String())
		return fn(c)
	}, nil
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

func applyWasmFilter(rew *Forward, c *fiber.Ctx) (err error) {
	r1 := extractRequestInfo(c)
	r2, err := executeWasm(r1, rew, c)
	if err != nil {
		return
	}
	changes, err := diffRequests(r1, r2)
	if err != nil {
		return
	}
	for _, ch := range *changes {
		switch ch.Path[0] {
		case "Method":
			c.Method(ch.To.(string))
		case "Path":
			c.Path(ch.To.(string))
		case "Headers":
			if ch.Type == "create" {
				for _, v := range ch.To.(shared.RequestParamsMultiValues) {
					c.Request().Header.Add(ch.Path[1], v)
				}
			}
			if ch.Type == "delete" {
				c.Request().Header.Del(ch.Path[1])
			}
		case "Query":
			if ch.Type == "create" {
				for _, v := range ch.To.(shared.RequestParamsMultiValues) {
					c.Context().QueryArgs().Add(ch.Path[1], v)
				}
			}
			if ch.Type == "delete" {
				c.Context().QueryArgs().Del(ch.Path[1])
			}
		case "Cookies":
			if ch.Type == "create" {
				c.Request().Header.SetCookie(ch.Path[1], ch.To.(string))
			}
			if ch.Type == "delete" {
				c.Request().Header.DelCookie(ch.Path[1])
			}
		}
	}
	return
}

func diffRequests(r1 *shared.RequestInfo, r2 *shared.RequestInfo) (*diff.Changelog, error) {
	changelog, err := diff.Diff(r1, r2)
	return &changelog, err
}

func extractRequestInfo(c *fiber.Ctx) (r *shared.RequestInfo) {
	r = &shared.RequestInfo{}
	r.Method = c.Method()
	r.Path = c.Path()
	r.Query = make(shared.RequestParamsMulti)
	r.Headers = make(shared.RequestParamsMulti)
	r.Cookies = make(shared.RequestParams)

	c.Context().QueryArgs().VisitAll(func(key, value []byte) {
		k := string(key)
		v := string(value)
		_, ok := r.Query[k]
		if ok {
			r.Query[k] = append(r.Query[k], v)
		} else {
			r.Query[k] = []string{v}
		}
	})

	h := &c.Request().Header

	h.VisitAll(func(key, value []byte) {
		k := string(key)
		v := string(value)
		_, ok := r.Headers[k]
		if ok {
			r.Headers[k] = append(r.Headers[k], v)
		} else {
			r.Headers[k] = []string{v}
		}
	})

	h.VisitAllCookie(func(key, value []byte) {
		r.Cookies[string(key)] = string(value)
	})

	return
}

func executeWasm(req *shared.RequestInfo, f *Forward, c *fiber.Ctx) (res *shared.RequestInfo, err error) {
	ctx := c.Context()
	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx) // This closes everything this Runtime created.

	// Instantiate WASI, which implements host functions needed for TinyGo to
	// implement `panic`.
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	// Instantiate the guest Wasm into the same runtime. It exports the `add`
	// function, implemented in WebAssembly.
	mod, err := r.Instantiate(ctx, f.WasmData)
	if err != nil {
		return
	}

	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		err = fmt.Errorf("Invalid alloc type")
		return
	}

	filter := mod.ExportedFunction("filter")
	if filter == nil {
		err = fmt.Errorf("Invalid filter type")
		return
	}

	reqJson, err := gojay.Marshal(req)
	if err != nil {
		return
	}

	reqSize := uint64(len(reqJson))

	allocRes, err := alloc.Call(ctx, reqSize)
	if err != nil {
		err = fmt.Errorf("failed to alloc memory: %v", err)
		return
	}

	reqPtr := allocRes[0]

	if ok := mod.Memory().Write(uint32(reqPtr), reqJson); !ok {
		err = fmt.Errorf("failed to write memory")
		return
	}

	resPtrSize, err := filter.Call(ctx, reqPtr, reqSize)
	if err != nil {
		err = fmt.Errorf("cannot call filter: %v", err)
		return
	}

	resPrt := uint32(resPtrSize[0] >> 32)
	resSize := uint32(resPtrSize[0])

	resJson, ok := mod.Memory().Read(resPrt, resSize)
	if !ok {
		err = fmt.Errorf("cannot read memory")
		return
	}

	res = &shared.RequestInfo{}
	err = gojay.Unmarshal(resJson, res)
	if err != nil {
		return
	}

	return
}
