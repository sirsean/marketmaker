package model

import (
	exchange "github.com/preichenberger/go-coinbase-exchange"
	"fmt"
	"log"
	"math"
	"sync"
	"code.google.com/p/go-uuid/uuid"
	"time"
)

type MyOrders struct {
	sync.RWMutex
	client *exchange.Client
	book *LocalBook
	availableBtc float64
	availableUsd float64
	pendingBuys map[string]exchange.Order
	pendingSells map[string]exchange.Order
	myBuys map[string]exchange.Order
	mySells map[string]exchange.Order
}

func NewMyOrders(client *exchange.Client, book *LocalBook) *MyOrders {
	return &MyOrders{
		client: client,
		book: book,
		pendingBuys: make(map[string]exchange.Order),
		pendingSells: make(map[string]exchange.Order),
		myBuys: make(map[string]exchange.Order),
		mySells: make(map[string]exchange.Order),
	}
}

func (mo *MyOrders) StartTicking() {
	accountTick := time.NewTicker(time.Second * 3).C
	ordersTick := time.NewTicker(time.Second * 60).C
	printTick := time.NewTicker(time.Second * 3).C
	//protectTick := time.NewTicker(time.Second * 20).C
	refillTick := time.NewTicker(time.Second * 10).C

	for {
		select {
			case <- accountTick:
				mo.RefreshAccount()
			case <- ordersTick:
				mo.RefreshOrders()
			//case <- protectTick:
			case <- refillTick:
				mo.ProtectBuys()
				mo.ProtectAsks()
				mo.RefillBids()
				mo.RefillAsks()
			case <- printTick:
				log.Printf("%v", mo)
		}
	}
}

func (mo *MyOrders) RefreshAccount() {
	accounts, err := mo.client.GetAccounts()
	if err != nil {
		log.Printf("failed to get accounts: %v", err)
		return
	}
	mo.Lock()
	for _, a := range accounts {
		if a.Currency == "BTC" {
			mo.availableBtc = a.Available
		} else if a.Currency == "USD" {
			mo.availableUsd = a.Available
		}
	}
	mo.Unlock()
}

func (mo *MyOrders) RefreshOrders() {
	log.Printf("refreshing orders")
	var page []exchange.Order
	cursor := mo.client.ListOrders()

	orders := make([]exchange.Order, 0)
	for cursor.HasMore {
		if err := cursor.NextPage(&page); err != nil {
			break
		}

		orders = append(orders, page...)
	}

	mo.Lock()
	for id := range mo.myBuys {
		delete(mo.myBuys, id)
	}
	for id := range mo.mySells {
		delete(mo.mySells, id)
	}
	for _, o := range orders {
		if o.Side == "buy" {
			mo.myBuys[o.Id] = o
		} else if o.Side == "sell" {
			mo.mySells[o.Id] = o
		}
	}
	mo.Unlock()
}

func (mo *MyOrders) CancelAllOrders() {
	log.Printf("cancel all my orders")
	orderIds := make([]string, 0)
	usd, btc := 0.0, 0.0
	mo.RLock()
	for id, o := range mo.myBuys {
		orderIds = append(orderIds, id)
		usd += o.Price * o.Size
	}
	for id, o := range mo.mySells {
		orderIds = append(orderIds, id)
		btc += o.Size
	}
	mo.RUnlock()
	mo.updateAvailableUsd(usd)
	mo.updateAvailableBtc(btc)
	if len(orderIds) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(orderIds))
		for _, id := range orderIds {
			go func(wg *sync.WaitGroup, id string) {
				log.Printf("canceling order %v", id)
				mo.client.CancelOrder(id)
				mo.removeBuy(id)
				mo.removeSell(id)
				wg.Done()
			}(&wg, id)
		}
		wg.Wait()
	}
}

func (mo *MyOrders) RefillBids() {
	// starting at this price, increment through 5 cents
	// check if we have a bid for that amount
	// if not, place a buy order
	price := mo.book.BestBidPrice()
	orders := make([]exchange.Order, 0)
	size := 0.01 // TODO temp conservative size
	if size >= 0.01 {
		current := price
		for x := 0; mo.totalBuyValue() < mo.currentBtcValue() / 2; x++ {
			order := exchange.Order{
				ClientOID: uuid.New(),
				Price: roundPlus(current, 2),
				Size: roundPlus(size, 8),
				Side: "buy",
				ProductId: "BTC-USD",
			}
			if mo.getAvailableUsd() >= order.Price * order.Size {
				mo.addPendingBuy(order)
				orders = append(orders, order)
				mo.updateAvailableUsd(-1 * order.Price * order.Size)
			} else {
				break
			}
			current -= 0.01 * float64(x % 3)
		}
		if len(orders) > 0 {
			var wg sync.WaitGroup
			wg.Add(len(orders))
			for _, o := range orders {
				go func(wg *sync.WaitGroup, o exchange.Order) {
					log.Printf("placing bid %0.4f @ %0.2f", o.Size, o.Price)
					_, err := mo.client.CreateOrder(&o)
					if err != nil {
						log.Printf("failed to place bid: %v", err)
						mo.removePendingBuy(o.ClientOID)
						mo.updateAvailableUsd(o.Price * o.Size)
					}
					wg.Done()
				}(&wg, o)
			}
			wg.Wait()
		}
	}
}

func (mo *MyOrders) HasBuyAtPrice(price float64) bool {
	mo.RLock()
	defer mo.RUnlock()
	for _, o := range mo.myBuys {
		if roundPlus(price, 2) == roundPlus(o.Price, 2) {
			return true
		}
	}
	return false
}

func (mo *MyOrders) RefillAsks() {
	// starting at this price, increment through 5 cents
	// check if we have an ask for that amount
	// if not, place a sell order
	price := mo.book.BestAskPrice()
	orders := make([]exchange.Order, 0)
	size := 0.01 // TODO temp conservative size
	if size >= 0.01 {
		current := price
		for x := 0; mo.totalSellValue() < mo.currentBtcValue() / 2; x++ {
			order := exchange.Order{
				ClientOID: uuid.New(),
				Price: roundPlus(current, 2),
				Size: roundPlus(size, 8),
				Side: "sell",
				ProductId: "BTC-USD",
			}
			if mo.getAvailableBtc() >= order.Size {
				mo.addPendingSell(order)
				orders = append(orders, order)
				mo.updateAvailableBtc(-1 * order.Size)
			} else {
				break
			}
			current += 0.01 * float64(x % 3)
		}
		if len(orders) > 0 {
			var wg sync.WaitGroup
			wg.Add(len(orders))
			for _, o := range orders {
				go func(wg *sync.WaitGroup, o exchange.Order) {
					log.Printf("placing ask %0.4f @ %0.2f (%v)", o.Size, o.Price, o.ClientOID)
					_, err := mo.client.CreateOrder(&o)
					if err != nil {
						log.Printf("failed to place ask: %v", err)
						mo.removePendingSell(o.ClientOID)
						mo.updateAvailableBtc(o.Size)
					}
					wg.Done()
				}(&wg, o)
			}
			wg.Wait()
		}
	}
}

func (mo *MyOrders) HasSellAtPrice(price float64) bool {
	mo.RLock()
	mo.RUnlock()
	for _, o := range mo.mySells {
		if roundPlus(price, 2) == roundPlus(o.Price, 2) {
			return true
		}
	}
	return false
}

func (mo *MyOrders) ProtectBuys() {
	// starting at the best bid
	// make sure we don't have any bids more than 5 cents away
	ordersToCancel := make([]exchange.Order, 0)
	bestBid := mo.book.BestBidPrice()
	mo.RLock()
	if mo.myBuys != nil {
		for _, o := range mo.myBuys {
			if o.Price < bestBid - 0.04 {
				log.Printf("canceling bid %v because %0.2f is too low", o.Id, o.Price)
				ordersToCancel = append(ordersToCancel, o)
			}
		}
	}
	mo.RUnlock()
	if len(ordersToCancel) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(ordersToCancel))
		for _, o := range ordersToCancel {
			go func(wg *sync.WaitGroup, o exchange.Order) {
				mo.removeBuy(o.Id)
				mo.updateAvailableUsd(o.Price * o.Size)
				mo.client.CancelOrder(o.Id)
				wg.Done()
			}(&wg, o)
		}
		wg.Wait()
	}
}

func (mo *MyOrders) ProtectAsks() {
	// starting at the best ask
	// make sure we don't have any asks more than 5 cents away
	ordersToCancel := make([]exchange.Order, 0)
	bestAsk := mo.book.BestAskPrice()
	mo.RLock()
	if mo.mySells != nil {
		for _, o := range mo.mySells {
			if o.Price > bestAsk + 0.04 {
				log.Printf("canceling ask %v because %0.2f is too high", o.Id, o.Price)
				ordersToCancel = append(ordersToCancel, o)
			}
		}
	}
	mo.RUnlock()
	if len(ordersToCancel) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(ordersToCancel))
		for _, o := range ordersToCancel {
			go func(wg *sync.WaitGroup, o exchange.Order) {
				mo.removeSell(o.Id)
				mo.updateAvailableBtc(o.Size)
				mo.client.CancelOrder(o.Id)
				wg.Done()
			}(&wg, o)
		}
		wg.Wait()
	}
}

func (mo *MyOrders) ReconcilePendingOrder(o *Order) {
	mo.RLock()
	buy, buyOk := mo.pendingBuys[o.ClientOID]
	sell, sellOk := mo.pendingSells[o.ClientOID]
	mo.RUnlock()
	if buyOk {
		mo.Lock()
		buy.Id = o.Id
		mo.myBuys[o.Id] = buy
		delete(mo.pendingBuys, o.ClientOID)
		mo.Unlock()
	}
	if sellOk {
		mo.Lock()
		sell.Id = o.Id
		mo.mySells[o.Id] = sell
		delete(mo.pendingSells, o.ClientOID)
		mo.Unlock()
	}
}

func (mo *MyOrders) ReconcileCanceledOrder(o *Order) {
	usd, btc := 0.0, 0.0
	mo.Lock()
	if _, ok := mo.myBuys[o.Id]; ok {
		delete(mo.myBuys, o.Id)
		usd += o.Size * o.Price
	}
	if _, ok := mo.mySells[o.Id]; ok {
		delete(mo.mySells, o.Id)
		btc += o.Size
	}
	mo.Unlock()
	mo.updateAvailableUsd(usd)
	mo.updateAvailableBtc(btc)
}

func (mo *MyOrders) ReconcileOrder(o *Order) (buy bool, sell bool) {
	buy = mo.reconcileBuys(o)
	sell = mo.reconcileSells(o)
	return
}

func (mo *MyOrders) reconcileBuys(o *Order) bool {
	mo.RLock()
	buy, ok := mo.myBuys[o.Id]
	mo.RUnlock()
	if ok {
		log.Printf("WE BOUGHT ONE (%0.2f)", o.Size)
	}
	if ok && o.Size <= 0.0 {
		log.Printf("adding the btc")
		mo.updateAvailableBtc(buy.Size)
		mo.removeBuy(o.Id)
		return true
	} else {
		return false
	}
}

func (mo *MyOrders) reconcileSells(o *Order) bool {
	mo.RLock()
	sell, ok := mo.mySells[o.Id]
	mo.RUnlock()
	if ok {
		log.Printf("WE SOLD ONE (%0.2f)", o.Size)
	}
	if ok && o.Size <= 0.0 {
		log.Printf("adding the usd")
		mo.updateAvailableUsd(sell.Size * sell.Price)
		mo.removeSell(o.Id)
		return true
	} else {
		return false
	}
}

func (mo *MyOrders) numBuys() int {
	mo.RLock()
	defer mo.RUnlock()
	return len(mo.myBuys) + len(mo.pendingBuys)
}

func (mo *MyOrders) addPendingBuy(o exchange.Order) {
	mo.Lock()
	mo.pendingBuys[o.ClientOID] = o
	mo.Unlock()
}

func (mo *MyOrders) removePendingBuy(clientOID string) {
	mo.Lock()
	delete(mo.pendingBuys, clientOID)
	mo.Unlock()
}

func (mo *MyOrders) numSells() int {
	mo.RLock()
	defer mo.RUnlock()
	return len(mo.mySells) + len(mo.pendingSells)
}

func (mo *MyOrders) addPendingSell(o exchange.Order) {
	mo.Lock()
	mo.pendingSells[o.ClientOID] = o
	mo.Unlock()
}

func (mo *MyOrders) removePendingSell(clientOID string) {
	mo.Lock()
	delete(mo.pendingSells, clientOID)
	mo.Unlock()
}

func (mo *MyOrders) removeBuy(id string) {
	mo.Lock()
	delete(mo.myBuys, id)
	mo.Unlock()
}

func (mo *MyOrders) removeSell(id string) {
	mo.Lock()
	delete(mo.mySells, id)
	mo.Unlock()
}

func (mo *MyOrders) getAvailableBtc() float64 {
	mo.RLock()
	defer mo.RUnlock()
	return mo.availableBtc
}

func (mo *MyOrders) getAvailableUsd() float64 {
	mo.RLock()
	defer mo.RUnlock()
	return mo.availableUsd
}

func (mo *MyOrders) updateAvailableBtc(amount float64) {
	mo.Lock()
	defer mo.Unlock()
	mo.availableBtc += amount
}

func (mo *MyOrders) updateAvailableUsd(amount float64) {
	mo.Lock()
	defer mo.Unlock()
	mo.availableUsd += amount
}

func (mo *MyOrders) totalBuyValue() float64 {
	mo.RLock()
	defer mo.RUnlock()
	totalBuy := 0.0
	for _, o := range mo.myBuys {
		totalBuy += o.Size
	}
	for _, o := range mo.pendingBuys {
		totalBuy += o.Size
	}
	return totalBuy
}

func (mo *MyOrders) totalSellValue() float64 {
	mo.RLock()
	defer mo.RUnlock()
	totalSell := 0.0
	for _, o := range mo.mySells {
		totalSell += o.Size
	}
	for _, o := range mo.pendingSells {
		totalSell += o.Size
	}
	return totalSell
}

func (mo *MyOrders) currentBtcValue() float64 {
	mo.RLock()
	defer mo.RUnlock()
	return mo.totalBuyValue() + mo.totalSellValue() + mo.availableBtc + (mo.availableUsd / mo.book.BestBidPrice())
}

func (mo *MyOrders) currentUsdValue() float64 {
	bestBid := mo.book.BestBidPrice()
	return mo.currentBtcValue() * bestBid
}

func (mo *MyOrders) String() string {
	mo.RLock()
	defer mo.RUnlock()
	totalBuy, totalSell := 0.0, 0.0
	for _, o := range mo.myBuys {
		totalBuy += o.Size
	}
	for _, o := range mo.mySells {
		totalSell += o.Size
	}
	usd := mo.availableUsd
	btc := mo.availableBtc
	bestBid := mo.book.BestBidPrice()
	buys := fmt.Sprintf("(%0.2f) ", bestBid)
	for _, o := range mo.myBuys {
		buys += fmt.Sprintf("%0.2f,", o.Price)
	}
	bestAsk := mo.book.BestAskPrice()
	sells := fmt.Sprintf("(%0.2f) ", bestAsk)
	for _, o := range mo.mySells {
		sells += fmt.Sprintf("%0.2f,", o.Price)
	}
	currentValueBtc := mo.currentBtcValue()
	currentValueUsd := mo.currentUsdValue()
	return fmt.Sprintf("buys: %v/%v, %0.4f, sells: %v/%v, %0.4f, USD: %0.4f, BTC: %0.4f\ncurrent account value: $%0.2f, %0.8fBTC\nbuys:  %v\nsells: %v", len(mo.myBuys), len(mo.pendingBuys), totalBuy, len(mo.mySells), len(mo.pendingSells), totalSell, usd, btc, currentValueUsd, currentValueBtc, buys, sells)
}

func round(f float64) float64 {
	return math.Floor(f + .5)
}

func roundPlus(f float64, places int) (float64) {
	shift := math.Pow(10, float64(places))
	return round(f * shift) / shift;
}
