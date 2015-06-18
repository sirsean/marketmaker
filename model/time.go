package model

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type Time struct {
	Iso string `json:"iso"`
	Epoch float64 `json:"epoch"`
}

func GetTime() (*Time, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://api.exchange.coinbase.com/time", nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	t := Time{}
	json.Unmarshal(data, &t)
	return &t, nil
}
