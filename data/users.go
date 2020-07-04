package data

import (
	"time"

	sub_data "elastic-mapping-generator/data/sub-data"
)

//go:generate elastic-mapping-generator ./users.go
// elastic:mappings
type User struct {
	Posts   sub_data.Posts `json:"posts"`
	Created time.Time
	User2
	Name string `json:"name"`
	Pass string `json:"pass" es:"analyzer:ik_smart"`
}

type User2 struct {
	Pass2 string `json:"pass2"`
}
