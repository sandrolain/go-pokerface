package scripts

import (
	"crypto/sha256"
	"fmt"

	"github.com/d5/tengo/v2"
)

type ScriptsManager struct {
	scripts map[string]*tengo.Compiled
}

func NewScriptsManager() *ScriptsManager {
	return &ScriptsManager{
		scripts: make(map[string]*tengo.Compiled),
	}
}

func (s *ScriptsManager) Prepare(source []byte) (hash string, err error) {
	h := sha256.New()
	h.Write(source)
	bs := h.Sum(nil)
	hash = fmt.Sprintf("%x\n", bs)

	if _, exist := s.scripts[hash]; exist {
		return
	}

	script := tengo.NewScript(source)

	compiled, err := script.Compile()
	if err != nil {
		return
	}

	s.scripts[hash] = compiled
	return
}

func (s *ScriptsManager) Get(hash string) (*tengo.Compiled, bool) {
	v, ok := s.scripts[hash]
	if ok {
		return v.Clone(), true
	}
	return nil, false
}
