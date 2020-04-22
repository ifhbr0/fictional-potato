package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	polo "github.com/ifhbr0/poloniex"
)

type tokenWatcher struct {
	token tradeToken

	ftxBidUpdate  chan float64
	poloAskUpdate chan float64
	bot           *Bot
}

type tradeToken struct {
	contractAddress string
	canonicalName   string
	altName         string
	poloEnabled     bool
}

type Bot struct {
	FtxLastPing     int64
	PoloLast        int64
	PoloUSDTBalance float64
	PoloClient      *polo.Poloniex
	Tokens          map[string]*tokenWatcher
	PricePerShare   map[string]float64
	ppsMut          sync.RWMutex
	noDeals         bool
}

func prefixTicker(token string) string {
	return "USDT_" + strings.ToUpper(token)
}

func postfixTicker(token string) string {
	return strings.ToUpper(token) + "/USDT"
}

func poloTicker(token tradeToken) string {
	return prefixTicker(token.canonicalName)
}

func NewTokenWatcher(token tradeToken, b *Bot) *tokenWatcher {
	t := tokenWatcher{}
	t.token = token
	t.bot = b
	t.ftxBidUpdate = make(chan float64)
	t.poloAskUpdate = make(chan float64)
	return &t
}

func (t *tokenWatcher) run() {
	var ask, bid, lastask, lastbid float64
	name := t.token.canonicalName

	for {
		select {
		case ask = <-t.poloAskUpdate:
			if t.token.canonicalName == "BEAR" && ask == 7.17 {
				log.Println(time.Now().UnixNano())
			}
			t.bot.PoloLast = time.Now().Unix()
			if ask == lastask {
				continue
			}
			lastask = ask
			//fmt.Println(t.token.canonicalName, "polo ask update ", ask)
			if t.bot.noDeals {
				continue
			}
			goodPrice := bid * 0.99
			t.bot.ppsMut.RLock()
			pps := t.bot.PricePerShare[name]
			t.bot.ppsMut.RUnlock()
			if goodPrice > ask && ask < pps && ask != 0 {

				t.bot.noDeals = true
				err := t.IOCBuyAndWd(name, goodPrice, (t.bot.PoloUSDTBalance-50.0)/goodPrice)
				if err != nil {
					log.Println(err)
				}
				fmt.Printf("BUY %s @ %.2f amount: %.2f (fair price: %.2f USD)\n", name, goodPrice, t.bot.PoloUSDTBalance/ask, pps)
				err = PoloUsdtBalanceUpdate(t.bot)
				for err != nil {
					log.Println(err)
					time.Sleep(time.Second)
					err = PoloUsdtBalanceUpdate(t.bot)
				}
			}

		case bid = <-t.ftxBidUpdate:
			if bid == lastbid {
				continue
			}
			lastbid = bid
			//fmt.Println("ftx bid update", bid)
			if t.bot.noDeals {
				continue
			}
			goodPrice := bid * 0.99
			t.bot.ppsMut.RLock()
			pps := t.bot.PricePerShare[name]
			t.bot.ppsMut.RUnlock()
			if goodPrice > ask && ask < pps && ask != 0 {

				t.bot.noDeals = true
				err := t.IOCBuyAndWd(name, goodPrice, (t.bot.PoloUSDTBalance-50.0)/goodPrice)
				if err != nil {
					log.Println(err)
				}

				fmt.Printf("BUY %s @ %.2f amount: %.2f (fair price: %.2f USD)\n", name, goodPrice, t.bot.PoloUSDTBalance/ask, pps)
				err = PoloUsdtBalanceUpdate(t.bot)
				for err != nil {
					log.Println(err)
					time.Sleep(time.Second)
					err = PoloUsdtBalanceUpdate(t.bot)
				}
			}
		}
	}
}

func NewBot(tokens []tradeToken) *Bot {
	b := Bot{}

	key, exists := os.LookupEnv("POLO_KEY")
	secret, secret_exists := os.LookupEnv("POLO_SECRET")

	if !exists || !secret_exists {
		log.Fatalln("polo key or secret not set")
	}
	poloniex, err := polo.NewClient(key, secret)
	if err != nil {
		log.Fatalln(err)
	}
	b.PoloClient = poloniex

	b.Tokens = make(map[string]*tokenWatcher)
	b.PricePerShare = make(map[string]float64)
	for _, t := range tokens {
		tw := NewTokenWatcher(t, &b)
		b.Tokens[t.canonicalName] = tw
	}
	return &b
}

func main() {

	b := NewBot(TokensConfig())

	go WatchFtxPrice(b)
	go watchPoloPrice(b)

	for _, t := range b.Tokens {
		go t.run()
		go t.UpdateShareInfoForever(100)
	}
	go BalanceUpdateForever(b)
	time.Sleep(10 * time.Second)
	sell, err := b.PoloClient.Sell("USDT_BEAR", 7.17, 1.0)
	log.Println("start", time.Now().UnixNano())
	log.Println(sell, err)

	for {
		time.Sleep(20 * time.Second)
		log.Println(b.FtxLastPing)
		log.Println("polo ", b.PoloLast)
	}
}

func TokensConfig() (tokens []tradeToken) {
	bull := tradeToken{
		"0x68eb95Dc9934E19B86687A10DF8e364423240E94",
		"BULL",
		"BTCBULL",
		true,
	}

	bear := tradeToken{
		"0x016ee7373248a80BDe1fD6bAA001311d233b3CFa",
		"BEAR",
		"BTCBEAR",
		true,
	}

	ethbull := tradeToken{
		"0x871baeD4088b863fd6407159f3672D70CD34837d",
		"ETHBULL",
		"ETHBULL",
		true,
	}

	ethbear := tradeToken{
		"0x2f5e2c9002C058c063d21A06B6cabb50950130c8",
		"ETHBEAR",
		"ETHBEAR",
		true,
	}
	/*
		eosbull := tradeToken{
			"0xeaD7F3ae4e0Bb0D8785852Cc37CC9d0B5e75c06a",
			"EOSBULL",
			"EOSBULL",
			false,
			true,
			true,
		}

		eosbear := tradeToken{
			"0x3d3dd61b0F9A558759a21dA42166042B114E12D5",
			"EOSBEAR",
			"EOSBEAR",
			false,
			true,
			true,
		} */
	bsvbear := tradeToken{
		"0xce49c3c92b33a1653f34811a9d7e34502bf12b89",
		"BSVBEAR",
		"BSVBEAR",
		true,
	}

	bsvbull := tradeToken{
		"0x6e13a9e4ae3d0678e511fb6d2ad531fcf0e247bf",
		"BSVBULL",
		"BSVBULL",
		true,
	}

	bchbull := tradeToken{
		"0x4c133e081dfb5858e39cca74e69bf603d409e57a",
		"BCHBULL",
		"BCHBULL",
		true,
	}

	bchbear := tradeToken{
		"0xa9fc65da36064ce545e87690e06f5de10c52c690",
		"BCHBEAR",
		"BCHBEAR",
		true,
	}
	/*
		xrpbull := tradeToken{
			"0x27c1bA4F85b8dC1c150157816623A6Ce80b7F187",
			"XRPBULL",
			"XRPBULL",
			true,
			true,
			true,
		}

		xrpbear := tradeToken{
			"0x94FC5934cF5970E944a67de806eEB5a4b493c6E6",
			"XRPBEAR",
			"XRPBEAR",
			true,
			true,
			true,
		} */

	//tokens = []tradeToken{bull, bear, ethbull, ethbear, eosbull, eosbear, xrpbull, xrpbear}
	tokens = []tradeToken{bull, bear, bsvbull, bsvbear, ethbull, ethbear, bchbear, bchbull} //, xrpbull, xrpbear}
	return
}
