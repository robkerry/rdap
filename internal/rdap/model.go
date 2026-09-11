package rdap

import (
	"encoding/json"
	"strings"
	"time"
)

type Link struct {
	Value string `json:"value"`
	Rel   string `json:"rel"`
	Href  string `json:"href"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type Event struct {
	Action string `json:"eventAction"`
	Date   string `json:"eventDate"`
	Actor  string `json:"eventActor"`
}

type Nameserver struct {
	LDHName     string `json:"ldhName"`
	UnicodeName string `json:"unicodeName"`
}

type Entity struct {
	Handle     string          `json:"handle"`
	Roles      []string        `json:"roles"`
	VCardArray json.RawMessage `json:"vcardArray"`
	Entities   []Entity        `json:"entities"`
	Links      []Link          `json:"links"`
}

type Domain struct {
	ObjectClassName string       `json:"objectClassName"`
	Handle          string       `json:"handle"`
	LDHName         string       `json:"ldhName"`
	UnicodeName     string       `json:"unicodeName"`
	Status          []string     `json:"status"`
	Nameservers     []Nameserver `json:"nameservers"`
	Events          []Event      `json:"events"`
	Entities        []Entity     `json:"entities"`
	Links           []Link       `json:"links"`
}

func (d Domain) EventDate(actions ...string) string {
	for _, action := range actions {
		for _, event := range d.Events {
			if strings.EqualFold(event.Action, action) && event.Date != "" {
				return event.Date
			}
		}
	}
	return ""
}

func ParseTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func EntityName(entity Entity) string {
	if len(entity.VCardArray) == 0 {
		return ""
	}

	var card []any
	if err := json.Unmarshal(entity.VCardArray, &card); err != nil || len(card) < 2 {
		return ""
	}

	props, ok := card[1].([]any)
	if !ok {
		return ""
	}

	for _, propRaw := range props {
		prop, ok := propRaw.([]any)
		if !ok || len(prop) < 4 {
			continue
		}
		name, _ := prop[0].(string)
		if !strings.EqualFold(name, "fn") {
			continue
		}
		value, _ := prop[3].(string)
		return value
	}

	return ""
}

func RegistrarName(d Domain) string {
	for _, entity := range d.Entities {
		for _, role := range entity.Roles {
			if strings.EqualFold(role, "registrar") {
				if name := EntityName(entity); name != "" {
					return name
				}
			}
		}
	}
	return ""
}
