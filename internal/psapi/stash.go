package psapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GGG API types

type RawStashTab struct {
	Id          string    `json:"id"`
	AccountName string    `json:"accountName"`
	League      string    `json:"league,omitempty"`
	Items       []RawItem `json:"items"`
	Name        string    `json:"stash"`
}

type RawItem struct {
	Id           string            `json:"id"`
	Name         string            `json:"name"`
	X            int               `json:"x"`
	Y            int               `json:"y"`
	Note         string            `json:"note"`
	Requirements []RawItemProperty `json:"requirements"`
	ExplicitMods []string          `json:"explicitMods"`
}

type RawItemProperty struct {
	Name   string `json:"name"`
	Values []RawItemPropValue
}

type RawItemPropValue struct {
	Value   string
	Numeric int // unsure what this field is used for, but we don't use it other than to unmarshal the data
}

func (i *RawItemPropValue) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &[]interface{}{&i.Value, &i.Numeric})
}

// application types

type StashSnapshot struct {
	Id         string
	League     string
	Items      []JewelEntry
	ChangeId   string
	RecordedAt time.Time
}

type JewelEntry struct {
	Id    string
	Type  string
	Class string
	Node  string
	Price Price
}

func (j JewelEntry) String() string {
	return fmt.Sprintf("%s (%s|%s) %s", j.Type, j.Class, j.Node, j.Id)
}

type Price struct {
	Count    float64
	Currency string
}

func FindFFJewels(r io.ReadCloser, l *log.Logger, changeId string) ([]StashSnapshot, error) {
	nodeRe := regexp.MustCompile("^Allocates (.+) if you have the matching modifier")
	timestamp := time.Now()
	tabsChecked := 0
	var stashes []StashSnapshot
	d := json.NewDecoder(r)

	// consume the opening brace
	if err := expect(d, json.Delim('{')); err != nil {
		return stashes, err
	}

	for d.More() {
		t, err := d.Token()
		if err != nil {
			return stashes, err
		}

		if t != "stashes" {
			if err := skip(d); err != nil {
				return stashes, err
			}
			continue
		}

		// stashes should be an array; consume the opening square bracket
		if err := expect(d, json.Delim('[')); err != nil {
			return stashes, err
		}

		for d.More() {
			var s RawStashTab
			if err := d.Decode(&s); err != nil {
				return stashes, err
			}
			tabsChecked++

			var jewels []JewelEntry
			for _, item := range s.Items {
				if item.Name == "Forbidden Flame" || item.Name == "Forbidden Flesh" {
					class := item.Requirements[0].Values[0].Value
					node := nodeRe.FindStringSubmatch(item.ExplicitMods[0])[1]
					price, err := FindPrice(&item, &s)

					// The item has no price listed
					if err != nil {
						continue
					}
					l.Printf("%s (%s|%s) found at price %f %s\n", item.Name, class, node, price.Count, price.Currency)

					j := JewelEntry{
						Id:    item.Id,
						Type:  item.Name,
						Class: class,
						Node:  node,
						Price: price,
					}

					jewels = append(jewels, j)
				}
			}

			stash := StashSnapshot{
				Id:         s.Id,
				League:     s.League,
				Items:      jewels,
				ChangeId:   changeId,
				RecordedAt: timestamp,
			}

			stashes = append(stashes, stash)
		}
	}
	return stashes, nil
}

func GetLatestChangeId(client *http.Client) (string, error) {
	type ChangeIdResponse struct {
		Psapi string `json:"psapi"`
	}

	req, err := http.NewRequest("GET", "https://www.pathofexile.com/api/trade/data/change-ids", nil)
	req.Header.Add("User-Agent", os.Getenv("GGG_USERAGENT"))
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data := ChangeIdResponse{}

	json.NewDecoder(resp.Body).Decode(&data)
	return data.Psapi, nil
}

func expect(d *json.Decoder, expectedT interface{}) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t != expectedT {
		return fmt.Errorf("got token %v, expected token %v", t, expectedT)
	}
	return nil
}

func skip(d *json.Decoder) error {
	nestLevel := 0
	for {
		t, err := d.Token()
		if err != nil {
			return err
		}

		switch t {
		case json.Delim('['), json.Delim('{'):
			nestLevel++
		case json.Delim(']'), json.Delim('}'):
			nestLevel++
		}

		if nestLevel == 0 {
			return nil
		}
	}
}

func FindPrice(item *RawItem, s *RawStashTab) (Price, error) {
	priceRe := regexp.MustCompile("^~price (.+)$")
	// first check the item note
	notePriceMatch := priceRe.FindStringSubmatch(item.Note)
	var match string
	if notePriceMatch != nil {
		match = notePriceMatch[1]
	}

	// second, check the stash tab name
	tabPriceMatch := priceRe.FindStringSubmatch(s.Name)
	if tabPriceMatch != nil {
		match = tabPriceMatch[1]
	}

	price, err := ParsePrice(match)
	if err != nil {
		return price, err
	}

	return price, nil
}

func ParsePrice(str string) (Price, error) {
	components := strings.Split(str, " ")
	if len(components) != 2 {
		return Price{}, errors.New("Could not parse price")
	}

	priceValue, err := strconv.ParseFloat(components[0], 64)
	if err != nil {
		return Price{}, err
	}

	return Price{priceValue, strings.TrimSpace(components[1])}, nil
}
