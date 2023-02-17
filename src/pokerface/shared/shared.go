package shared

import (
	"github.com/francoispqt/gojay"
)

type RequestParams map[string]string

func (m RequestParams) UnmarshalJSONObject(dec *gojay.Decoder, k string) error {
	str := ""
	if err := dec.String(&str); err != nil {
		return err
	}
	m[k] = str
	return nil
}

func (m RequestParams) NKeys() int {
	return 0
}

func (m RequestParams) MarshalJSONObject(enc *gojay.Encoder) {
	for k, v := range m {
		enc.StringKey(k, v)
	}
}

func (m RequestParams) IsNil() bool {
	return m == nil
}

type RequestParamsMulti map[string]RequestParamsMultiValues

func (m RequestParamsMulti) UnmarshalJSONObject(dec *gojay.Decoder, k string) error {
	str := RequestParamsMultiValues{}
	if err := dec.Array(&str); err != nil {
		return err
	}
	m[k] = str
	return nil
}

func (m RequestParamsMulti) NKeys() int {
	return 0
}

func (m RequestParamsMulti) MarshalJSONObject(enc *gojay.Encoder) {
	for k, v := range m {
		enc.ArrayKey(k, v)
	}
}

func (m RequestParamsMulti) IsNil() bool {
	return m == nil
}

type RequestParamsMultiValues []string

// implement UnmarshalerJSONArray
func (t *RequestParamsMultiValues) UnmarshalJSONArray(dec *gojay.Decoder) error {
	str := ""
	if err := dec.String(&str); err != nil {
		return err
	}
	*t = append(*t, str)
	return nil
}

func (u RequestParamsMultiValues) MarshalJSONArray(enc *gojay.Encoder) {
	for _, e := range u {
		enc.String(e)
	}
}
func (u RequestParamsMultiValues) IsNil() bool {
	return len(u) == 0
}

type RequestInfo struct {
	Method  string
	Path    string
	Query   RequestParamsMulti
	Cookies RequestParams
	Headers RequestParamsMulti
}

// implement gojay.UnmarshalerJSONObject
func (u *RequestInfo) UnmarshalJSONObject(dec *gojay.Decoder, key string) error {
	switch key {
	case "m":
		return dec.String(&u.Method)
	case "p":
		return dec.String(&u.Path)
	case "h":
		u.Headers = make(RequestParamsMulti)
		return dec.Object(&u.Headers)
	case "q":
		u.Query = make(RequestParamsMulti)
		return dec.Object(&u.Query)
	case "c":
		u.Cookies = make(RequestParams)
		return dec.Object(&u.Cookies)
	}
	return nil
}

func (u *RequestInfo) MarshalJSONObject(enc *gojay.Encoder) {
	enc.StringKey("m", u.Method)
	enc.StringKey("p", u.Path)
	enc.ObjectKey("h", u.Headers)
	enc.ObjectKey("q", u.Query)
	enc.ObjectKey("c", u.Cookies)
}

func (u *RequestInfo) IsNil() bool {
	return u == nil
}

func (u *RequestInfo) NKeys() int {
	return 0
}
