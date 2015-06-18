package model

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type OrderBook struct {
	Sequence int64 `json:"sequence"`
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

func (ob *OrderBook) BidOrders() []*Order {
	orders := make([]*Order, len(ob.Bids))
	for i, s := range ob.Bids {
		orders[i] = ParseOrder(s)
	}
	return orders
}

func (ob *OrderBook) AskOrders() []*Order {
	orders := make([]*Order, len(ob.Asks))
	for i, s := range ob.Asks {
		orders[i] = ParseOrder(s)
	}
	return orders
}

func DownloadOrderBook() (*OrderBook, error) {
	resp, err := http.Get("https://api.exchange.coinbase.com/products/BTC-USD/book?level=3")

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ob := OrderBook{}
	json.Unmarshal(data, &ob)
	return &ob, nil
}
