package ui

import (
	"strings"
)

func (u *UI) className(code string) string {
	for _, c := range u.model.Classes {
		if c.Code == code {
			return code + " — " + c.Name + "; decided by " + strings.ToLower(c.Arbiter[:1]) + c.Arbiter[1:]
		}
	}
	return code
}
