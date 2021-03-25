package jumpway

import (
	"strings"
)

type Way []string

func (w Way) String() string {
	return strings.Join(w, "|")
}

type Ways []Way

func (w Ways) Strings() []string {
	d := make([]string, 0, len(w))
	for _, v := range w {
		d = append(d, v.String())
	}
	return d
}
