package main

import (
	"encoding/json"
	"log"
	"net/http"
	"github.com/gorilla/websocket"
	"github.com/sirsean/marketmaker/config"
	"github.com/sirsean/marketmaker/model"
	exchange "github.com/preichenberger/go-coinbase-exchange"
	"os/signal"
	"os"
	"syscall"
)

var client *exchange.Client
var myOrders *model.MyOrders
var book *model.LocalBook
var msgChan chan model.Message
var buyChan chan *model.Order
var sellChan chan *model.Order
var sigChan chan os.Signal
var bidChangeChan chan *model.Order
var askChangeChan chan *model.Order

func main() {
	log.Printf("starting up")

	client = exchange.NewClient(
		config.Get().Coinbase.Secret,
		config.Get().Coinbase.Key,
		config.Get().Coinbase.Passphrase)

	msgChan = make(chan model.Message)
	buyChan = make(chan *model.Order)
	sellChan = make(chan *model.Order)
	sigChan = make(chan os.Signal)
	bidChangeChan = make(chan *model.Order)
	askChangeChan = make(chan *model.Order)
	book = model.NewLocalBook(bidChangeChan, askChangeChan)
	myOrders = model.NewMyOrders(client, book)

	myOrders.RefreshAccount()
	myOrders.RefreshOrders()
	go myOrders.StartTicking()

	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)
	go func(c chan os.Signal, mo *model.MyOrders) {
		<-c
		mo.CancelAllOrders()
		os.Exit(1)
	}(sigChan, myOrders)

	// subscribe
	conn := subscribe()
	defer conn.Close()

	go listenForMessages(conn, msgChan)
	go watchBuys(buyChan)
	go watchSells(sellChan)
	go watchBidChanges(bidChangeChan)
	go watchAskChanges(askChangeChan)

	initOrderBook()
	printInfo()

	myOrders.RefillBids()
	myOrders.RefillAsks()

	handleMessages()
}

func subscribe() *websocket.Conn {
	url := "wss://ws-feed.exchange.coinbase.com"
	wsHeaders := http.Header{}
	conn, _, err := websocket.DefaultDialer.Dial(url, wsHeaders)
	if err != nil {
		log.Fatal("websocket failed to connect: %v", err)
	}
	log.Printf("connected!")

	type Subscribe struct {
		Type string `json:"type"`
		ProductId string `json:"product_id"`
	}
	subscription := Subscribe{
		Type: "subscribe",
		ProductId: "BTC-USD",
	}
	msg, _ := json.Marshal(subscription)
	log.Printf("m: %v", string(msg))
	err = conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		log.Printf("failed to send message: %v", err)
	}
	log.Printf("sent subscription")

	return conn
}

func initOrderBook() {
	ob, err := model.DownloadOrderBook()
	if err != nil {
		log.Printf("failed to download order book: %v", err)
		return
	}
	log.Printf("downloaded order book. bids: %v, asks: %v", len(ob.Bids), len(ob.Asks))

	for _, o := range ob.BidOrders() {
		book.AddBid(o)
	}
	for _, o := range ob.AskOrders() {
		book.AddAsk(o)
	}
}

func handleMessages() {
	for msg := range msgChan {
		//log.Printf("%v", msg.String())
		if msg.IsReceived() {
			o := msg.Order()
			book.AddOrder(o)
			myOrders.ReconcilePendingOrder(o)
		} else if msg.IsOpen() {
			if o, ok := book.GetOrder(msg.OrderId); ok {
				if msg.IsBuy() {
					book.AddBid(o)
				} else if msg.IsSell() {
					book.AddAsk(o)
				}
			}
		} else if msg.IsDone() {
			if o, ok := book.GetOrder(msg.OrderId); ok {
				if msg.IsBuy() {
					book.RemoveBid(o)
				} else if msg.IsSell() {
					book.RemoveAsk(o)
				}
				if msg.IsCanceled() {
					myOrders.ReconcileCanceledOrder(o)
				} else {
					myOrders.ReconcileOrder(o)
				}
			}
		} else if msg.IsMatch() {
			_, _, taker, _ := book.HandleMatch(msg)
			if msg.IsBuy() {
				buyChan <- taker
			} else if msg.IsSell() {
				sellChan <- taker
			}
		}
	}
}

func listenForMessages(conn *websocket.Conn, msgChan chan model.Message) {
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Printf("failed to read message: %v", err)
		}
		message := model.Message{}
		json.Unmarshal(raw, &message)
		//log.Printf(string(raw))
		msgChan <- message
	}
}

func printInfo() {
	log.Printf("%v", book)
	log.Printf("MO: %v", myOrders)
}

func watchBuys(c chan *model.Order) {
	for o := range c {
		log.Printf("BUY! %v", o.Price)
		go refillMyOrders()
	}
}

func watchSells(c chan *model.Order) {
	for o := range c {
		log.Printf("SELL! %v", o.Price)
		go refillMyOrders()
	}
}

func watchBidChanges(c chan *model.Order) {
	for o := range c {
		log.Printf("BID CHANGED: %v", o.Price)
		go refillMyOrders()
	}
}

func watchAskChanges(c chan *model.Order) {
	for o := range c {
		log.Printf("ASK CHANGED: %v", o.Price)
		go refillMyOrders()
	}
}

func refillMyOrders() {
	myOrders.ProtectBuys()
	myOrders.ProtectAsks()
	myOrders.RefillBids()
	myOrders.RefillAsks()
	printInfo()
}
