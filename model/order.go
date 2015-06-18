package model

import (
	"fmt"
	"strconv"
)

type Order struct {
	Price float64
	Size float64
	Id string
	ClientOID string
}

func ParseOrder(parts []string) *Order {
	price, _ := strconv.ParseFloat(parts[0], 64)
	size, _ := strconv.ParseFloat(parts[1], 64)
	return &Order{
		Price: price,
		Size: size,
		Id: parts[2],
	}
}

func (o *Order) String() string {
	return fmt.Sprintf("{Order Id: %v, Price: %v, Size: %v}", o.Id, o.Price, o.Size)
}
