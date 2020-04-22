package main

import (
	"errors"
	"log"
	"time"

	polo "github.com/ifhbr0/poloniex"
)

func watchPoloPrice(b *Bot) {
	ws, err := polo.NewWSClient()
	if err != nil {
		return
	}

	err = ws.SubscribeTicker()
	if err != nil {
		return
	}

	for {
		select {
		case ticker := <-ws.Subs["TICKER"]:
			t, ok := ticker.(polo.WSTicker)
			if !ok {
				continue
			}

			for _, tok := range b.Tokens {
				if poloTicker(tok.token) == t.Symbol {
					b.Tokens[tok.token.canonicalName].poloAskUpdate <- t.LowestAsk
					continue
				}
			}
		default:
			//fmt.Println(<-ws.Subs["TICKER"])
		}
	}
}

func BalanceUpdateForever(b *Bot) {
	for {
		err := PoloUsdtBalanceUpdate(b)
		if err != nil {
			log.Println(err)
			continue
		}
		time.Sleep(10 * time.Second)

	}
}

func PoloUsdtBalanceUpdate(b *Bot) (err error) {
	resp, err := b.PoloClient.GetAccountBalances()
	if err != nil {
		return
	}
	balance, _ := resp.Exchange["USDT"].Float64()
	if balance < 100.0 {
		b.noDeals = true
		log.Printf("low usdt balance, global stop")
	} else {
		if b.noDeals {
			b.noDeals = false
		}
	}
	b.PoloUSDTBalance = balance

	return
}

func (t *tokenWatcher) IOCBuyAndWd(tokenName string, price, amount float64) (err error) {
	currencyPair := "USDT_" + tokenName
	ftxDepositAddress := "0x6D893c3e866D2Bb7876BaeC3CEd305897ed28822"
	resp, err := t.bot.PoloClient.IOCBuy(currencyPair, price, amount)
	if err != nil {

		return
	}
	//log.Println(resp)
	if len(resp.ResultingTrades) == 0 {
		err = errors.New("ioc buy failed - no trades")
		return
	}
	accounts, err := t.bot.PoloClient.GetAccountBalances()
	if err != nil {

		return
	}

	balance, _ := accounts.Exchange[t.token.canonicalName].Float64()
	if balance*price > 50.0 {
		wd, err := t.bot.PoloClient.Withdraw(t.token.canonicalName, ftxDepositAddress, balance)
		if err != nil {

			return err
		}

		log.Println(wd)
	}
	return
}
