package model

import (
	"fmt"
	"sort"
	"sync"
)

type Asks struct {
	sync.RWMutex
	orders []*Order
}

func NewAsks() *Asks {
	return &Asks{
		orders: make([]*Order, 0),
	}
}

func (b *Asks) String() string {
	b.RLock()
	defer b.RUnlock()
	return fmt.Sprintf("%v", b.orders)
}

func (b *Asks) Best() *Order {
	b.RLock()
	defer b.RUnlock()
	if len(b.orders) > 0 {
		return b.orders[0]
	} else {
		return nil
	}
}

func (b *Asks) Add(o *Order) {
	b.Lock()
	defer b.Unlock()
	b.orders = append(b.orders, o)
	sort.Sort(b)
}

func (b *Asks) IndexOf(o *Order) int {
	b.RLock()
	defer b.RUnlock()
	for i, order := range b.orders {
		if o == order {
			return i
		}
	}
	return -1
}

func (b *Asks) Remove(o *Order) {
	index := b.IndexOf(o)
	b.Lock()
	defer b.Unlock()
	if index != -1 {
		b.orders = append(b.orders[:index], b.orders[index+1:]...)
	}
}

func (b *Asks) Len() int {
	return len(b.orders)
}

func (b *Asks) Less(i, j int) bool {
	if b.orders[i].Price == b.orders[j].Price {
		return b.orders[i].Size > b.orders[j].Size
	} else {
		return b.orders[i].Price < b.orders[j].Price
	}
}

func (b *Asks) Swap(i, j int) {
	b.orders[i], b.orders[j] = b.orders[j], b.orders[i]
}
