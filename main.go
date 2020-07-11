package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/sunabozu/binance-price-change-go/utils"

	"github.com/adshao/go-binance"
)

func main() {
	parentPath, err := utils.GetParentPath()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	keys, err := utils.LoadKeys(parentPath + "/env.json")

	if err != nil {
		os.Exit(1)
	}

	client := binance.NewClient(keys.BinanceKey, keys.BinanceSecret)
	listenKey, err := client.NewStartUserStreamService().Do(context.Background())

	// update the listenKey evey once in a while, see https://github.com/binance-exchange/binance-official-api-docs/blob/master/user-data-stream.md#pingkeep-alive-a-listenkey
	go func() {
		for {
			time.Sleep(time.Minute * 10)
			log.Println("updating the listenKey...")
			err := client.NewKeepaliveUserStreamService().Do(context.Background())

			if err != nil {
				log.Println(err)
			}
		}
	}()

	log.Printf("%+v", listenKey)

	wsHandler := func(message []byte) {
		var userData map[string]interface{}

		if err := json.Unmarshal(message, &userData); err != nil {
			log.Println(err)
			return
		}

		log.Println(userData["e"], userData["X"], userData["x"])

		if userData["e"] != "executionReport" || userData["X"] != "FILLED" {
			return
		}

		price, err := strconv.ParseFloat(userData["p"].(string), 64)
		quantity, err := strconv.ParseFloat(userData["q"].(string), 64)

		if err != nil {
			return
		}

		usdValue := quantity * price
		symbol := userData["s"].(string)
		side := userData["S"].(string)
		currency := "฿"
		emoji := "⬇️⬇️⬇️"
		priceStr := fmt.Sprintf("%.2f", price)
		usdValueStr := fmt.Sprintf("%.2f", usdValue)

		if symbol[len(symbol)-4:] == "USDT" {
			symbol = symbol[0 : len(symbol)-4]
			currency = "$"
		} else if symbol[len(symbol)-3:] == "BTC" {
			symbol = symbol[0 : len(symbol)-3]
			priceStr = fmt.Sprintf("%.8f", price)
			usdValueStr = fmt.Sprintf("%.8f", usdValue)
		}

		content := fmt.Sprintf("%.6f %s (%s%s)", quantity, symbol, currency, usdValueStr)

		if side == "BUY" {
			content += " was BOUGHT"
		} else {
			content += " was SOLD"
			emoji = "✅✅✅"
		}

		content += fmt.Sprintf(" at the price of %s %s", priceStr, emoji)

		log.Println(content)

		go utils.SendPushNotification(keys, content)
	}

	errHandler := func(err error) {
		log.Printf("error in the handler: %+v", err)
	}

	doneC, _, err := binance.WsUserDataServe(listenKey, wsHandler, errHandler)
	log.Printf("a channel: %+v\n error: %+v\n", <-doneC, err)
}
