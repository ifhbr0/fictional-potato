package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type FtxTickerUpdate struct {
	Channel string `json:"channel"`
	Market  string `json:"market"`
	Type    string `json:"type"`
	Data    struct {
		Bid     float64 `json:"bid"`
		Ask     float64 `json:"ask"`
		BidSize float64 `json:"bidSize"`
		AskSize float64 `json:"askSize"`
		Last    float64 `json:"last"`
		Time    float64 `json:"time"`
	} `json:"data"`
}

func WatchFtxPrice(b *Bot) {
	address := fmt.Sprintf("wss://ftx.com/ws/")
	var wsDialer websocket.Dialer
	wsConn, _, err := wsDialer.Dial(address, nil)
	if err != nil {
		panic(err)
	}
	defer wsConn.Close()
	log.Println("Dialed:", address)

	for token := range b.Tokens {
		command := fmt.Sprintf("{\"op\": \"subscribe\", \"channel\": \"ticker\", \"market\": \"%s/USDT\"}", token)
		err = wsConn.WriteMessage(websocket.TextMessage, []byte(command))
		if err != nil {
			panic(err)
		}
	}

	go func() {
		t := time.NewTicker(time.Second * 15)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				err := wsConn.WriteMessage(websocket.TextMessage, []byte("{\"op\": \"ping\"}"))
				if err != nil {
					log.Printf("%v", err)
				}
			}
		}
	}()

	for {
		_, message, err := wsConn.ReadMessage()
		if err != nil {
			log.Println("[ERROR] ReadMessage:", err)
		}

		//log.Println(string(message))
		if string(message) == "{\"type\": \"pong\"}" {
			b.FtxLastPing = time.Now().Unix()
			continue
		}

		u := FtxTickerUpdate{}

		err = json.Unmarshal(message, &u)
		if err != nil {
			log.Println("[ERROR] Parsing:", err)
			continue

		}
		//fmt.Printf("%s, bid: %.2f \n", u.Market, u.Data.Bid)

		if len(u.Market) < 9 {
			log.Printf("bad market len: %s\n", u.Market)
			continue
		}
		b.Tokens[u.Market[:len(u.Market)-5]].ftxBidUpdate <- u.Data.Bid
	}
}

type ShareInfo struct {
	Result struct {
		PricePerShare  float64 `json: "pricePerShare"`
		UnderlyingMark float64 `json: "underlyingMark"`
	} `json: "result"`
	Success bool `json: "success"`
}

func GetShareInfo(tokenName string) (si ShareInfo, err error) {

	response, err := http.Get("https://ftx.com/api/lt/" + tokenName)
	if err != nil {
		return
	}
	defer response.Body.Close()
	dec := json.NewDecoder(response.Body)

	err = dec.Decode(&si)

	if err != nil {
		return
	}

	if si.Success != true {
		return si, errors.New("success was not true")
	}
	return si, nil
}

func (t *tokenWatcher) UpdateShareInfoForever(sleep int) {
	name := t.token.canonicalName
	log.Printf("started share info watcher %s\n", name)
	for {
		si, err := GetShareInfo(name)
		if err != nil {
			log.Println(err)
			continue
		}
		t.bot.ppsMut.Lock()
		t.bot.PricePerShare[name] = si.Result.PricePerShare
		t.bot.ppsMut.Unlock()
		time.Sleep(time.Millisecond * time.Duration(sleep))
	}
}

func (t *tokenWatcher) WaitAndSell(sleep int) {

}
