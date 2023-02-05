package forward

type Rewrite map[string]string

type Params map[string]string

type Forward struct {
	Method      string
	Path        string
	Rewrite     Rewrite
	Destination string
	Query       Params
	Headers     Params
}
