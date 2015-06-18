package model

import (
	//"log"
	"fmt"
	"sync"
)

type LocalBook struct {
	sync.RWMutex
	book map[string]*Order
	bids *Bids
	asks *Asks
	lastPrice float64
	spread float64
	bestBidPrice float64
	bestAskPrice float64
	bidChangeChan chan *Order
	askChangeChan chan *Order
}

func NewLocalBook(bidChangeChan chan *Order, askChangeChan chan *Order) *LocalBook {
	return &LocalBook{
		book: make(map[string]*Order),
		bids: NewBids(),
		asks: NewAsks(),
		bidChangeChan: bidChangeChan,
		askChangeChan: askChangeChan,
	}
}

func (b *LocalBook) recalculateSpread() {
	oldBestBidPrice := b.bestBidPrice
	oldBestAskPrice := b.bestAskPrice
	oldSpread := b.spread
	bid := b.bids.Best()
	ask := b.asks.Best()
	if bid != nil {
		b.bestBidPrice = bid.Price
	}
	if ask != nil {
		b.bestAskPrice = ask.Price
	}
	if bid != nil && ask != nil {
		b.spread =  ask.Price - bid.Price
	} else {
		b.spread =  -1
	}
	if oldSpread != b.spread {
		//log.Printf("SPREAD CHANGED")
	}
	if oldBestBidPrice != b.bestBidPrice {
		b.bidChangeChan <- bid
	}
	if oldBestAskPrice != b.bestAskPrice {
		b.askChangeChan <- ask
	}
}

func (b *LocalBook) BestBidPrice() float64 {
	b.RLock()
	defer b.RUnlock()
	return b.bestBidPrice
}

func (b *LocalBook) BestAskPrice() float64 {
	b.RLock()
	defer b.RUnlock()
	return b.bestAskPrice
}

func (b *LocalBook) GetOrder(id string) (*Order, bool) {
	b.RLock()
	defer b.RUnlock()
	o, ok := b.book[id]
	return o, ok
}

func (b *LocalBook) AddOrder(o *Order) {
	b.Lock()
	defer b.Unlock()
	b.book[o.Id] = o
}

func (b *LocalBook) AddBid(o *Order) {
	b.AddOrder(o)
	b.bids.Add(o)
	b.recalculateSpread()
}

func (b *LocalBook) AddAsk(o *Order) {
	b.AddOrder(o)
	b.asks.Add(o)
	b.recalculateSpread()
}

func (b *LocalBook) removeOrder(o *Order) {
	b.Lock()
	delete(b.book, o.Id)
	b.Unlock()
}

func (b *LocalBook) RemoveBid(o *Order) {
	b.bids.Remove(o)
	b.removeOrder(o)
	b.recalculateSpread()
}

func (b *LocalBook) RemoveAsk(o *Order) {
	b.asks.Remove(o)
	b.removeOrder(o)
	b.recalculateSpread()
}

func (b *LocalBook) HandleMatch(msg Message) (*Order, bool, *Order, bool) {
	maker, makerOk := b.GetOrder(msg.MakerOrderId)
	taker, takerOk := b.GetOrder(msg.TakerOrderId)
	b.Lock()
	b.lastPrice = msg.ParsedPrice()
	if makerOk {
		maker.Size -= msg.ParsedSize()
	}
	if takerOk {
		taker.Size -= msg.ParsedSize()
	}
	b.Unlock()
	return maker, makerOk, taker, takerOk
}

func (b *LocalBook) String() string {
	var bid, bidSize, ask, askSize float64
	bestBid := b.bids.Best()
	if bestBid != nil {
		bid = bestBid.Price
		bidSize = bestBid.Size
	}
	bestAsk := b.asks.Best()
	if bestAsk != nil {
		ask = bestAsk.Price
		askSize = bestAsk.Size
	}
	b.RLock()
	defer b.RUnlock()
	return fmt.Sprintf("last: %v, spread: %0.2f, bid: %0.4f@%0.2f, ask: %0.4f@%0.2f, book size: %v", b.lastPrice, b.spread, bidSize, bid, askSize, ask, len(b.book))
}
