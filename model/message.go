package model

import (
	"fmt"
	"strconv"
)

type Message struct {
	Type string `json:"type"`
	Time string `json:"time"`
	ProductId string `json:"product_id"`
	Sequence int64 `json:"sequence"`
	OrderId string `json:"order_id"`
	Size string `json:"size"`
	Price string `json:"price"`
	Side string `json:"side"`
	// received
	ClientOID string `json:"client_oid"`
	// open, done
	RemainingSize string `json:"remaining_size"`
	// done
	Reason string `json:"reason"`
	// match
	TradeId int64 `json:"trade_id"`
	MakerOrderId string `json:"maker_order_id"`
	TakerOrderId string `json:"taker_order_id"`
	// change
	NewSize string `json:"new_size"`
	OldSize string `json:"old_size"`
}

func (m *Message) ParsedSize() float64 {
	s, _ := strconv.ParseFloat(m.Size, 64)
	return s
}

func (m *Message) ParsedPrice() float64 {
	p, _ := strconv.ParseFloat(m.Price, 64)
	return p
}

func (m *Message) IsReceived() bool {
	return m.Type == "received"
}

func (m *Message) IsOpen() bool {
	return m.Type == "open"
}

func (m *Message) IsDone() bool {
	return m.Type == "done"
}

func (m *Message) IsMatch() bool {
	return m.Type == "match"
}

func (m *Message) IsBuy() bool {
	return m.Side == "buy"
}

func (m *Message) IsSell() bool {
	return m.Side == "sell"
}

func (m *Message) IsCanceled() bool {
	return m.Reason == "canceled"
}

func (m *Message) IsFilled() bool {
	return m.Reason == "filled"
}

func (m *Message) Order() *Order {
	return &Order{
		Id: m.OrderId,
		ClientOID: m.ClientOID,
		Size: m.ParsedSize(),
		Price: m.ParsedPrice(),
	}
}

func (m *Message) String() string {
	if m.Type == "received" {
		return fmt.Sprintf("%v: %v %v %v %v (%v)", m.Type, m.Side, m.Price, m.Size, m.OrderId, m.ClientOID)
	} else if m.Type == "open" {
		return fmt.Sprintf("%v: %v %v %v %v", m.Type, m.Side, m.Price, m.Size, m.OrderId)
	} else if m.Type == "done" {
		return fmt.Sprintf("%v: %v %v %v %v %v", m.Type, m.Side, m.Reason, m.Price, m.Size, m.OrderId)
	} else if m.Type == "match" {
		return fmt.Sprintf("%v: %v %v %v %v %v", m.Type, m.Side, m.Price, m.Size, m.MakerOrderId, m.TakerOrderId)
	} else if m.Type == "change" {
		return fmt.Sprintf("%v: %v %v %v %v", m.Type, m.Side, m.Price, m.NewSize, m.OldSize)
	} else {
		return fmt.Sprintf("unknown %v", m.Type)
	}
}
