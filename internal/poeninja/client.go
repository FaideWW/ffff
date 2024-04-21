package poeninja

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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

func GetLatestPSChangeId(client *http.Client) (string, error) {
	type ChangeIdResponse struct {
		NextChangeId string `json:"next_change_id"`
	}

	req, _ := http.NewRequest("GET", "https://poe.ninja/api/data/getstats", nil)
	// req.Header.Add("User-Agent", os.Getenv("GGG_USERAGENT"))
	resp, err := client.Do(req)
	// bodyBytes, _ := io.ReadAll(resp.Body)
	// fmt.Printf("%+v\n", string(bodyBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data := ChangeIdResponse{}

	json.NewDecoder(resp.Body).Decode(&data)
	return data.NextChangeId, nil
}

// poe.ninja API types

type RawCurrencyResponse struct {
	Lines           []RawCurrencyLine   `json:"lines"`
	CurrencyDetails []RawCurrencyDetail `json:"currencyDetails"`
}

type RawCurrencyLine struct {
	CurrencyTypeName string  `json:"CurrencyTypeName"`
	ChaosEquivalent  float64 `json:"chaosEquivalent"`
}

type RawCurrencyDetail struct {
	Name    string `json:"name"`
	TradeId string `json:"tradeId"`
}

func GetExchangeRates(client *http.Client, league string) (map[string]float64, error) {
	start := time.Now()
	fmt.Printf("fetching currency data from poe.ninja\n")
	req, _ := http.NewRequest("GET", "https://poe.ninja/api/data/currencyoverview?league="+league+"&type=Currency", nil)
	// req.Header.Add("User-Agent", os.Getenv("GGG_USERAGENT"))
	resp, err := client.Do(req)
	// bodyBytes, _ := io.ReadAll(resp.Body)
	// fmt.Printf("%+v\n", string(bodyBytes))
	if err != nil {
		fmt.Printf("failed to fetch: %s", err)
		return nil, err
	}
	defer resp.Body.Close()
	networkElapsed := time.Since(start)

	d := json.NewDecoder(resp.Body)

	var data RawCurrencyResponse
	d.Decode(&data)

	rates := make(map[string]float64, len(data.Lines))

	tradeIdMap := make(map[string]string, len(data.CurrencyDetails))
	for _, row := range data.CurrencyDetails {
		if row.TradeId != "" {
			tradeIdMap[row.Name] = row.TradeId
		}
	}

	for _, row := range data.Lines {
		if tradeId, ok := tradeIdMap[row.CurrencyTypeName]; ok {
			rates[tradeId] = row.ChaosEquivalent
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("Retrieved exchange rates in %dms (network took %dms)\n", elapsed.Milliseconds(), networkElapsed.Milliseconds())

	return rates, nil
}
