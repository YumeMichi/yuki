package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/anaskhan96/soup"
)

var (
	idolURL   = "https://idol.st"
	cardsApi  = "https://idol.st/ajax/allstars/cards/?page="
	cardsPath = "cards"

	maxChanCount = 10
)

var httpProxy = flag.String("p", "", "HTTP Proxy URL (eg: http://127.0.0.1:7890)")

type ImageInfo struct {
	URL  string
	Idol string
}

func init() {
	_, err := os.Stat(cardsPath)
	if err != nil {
		err = os.Mkdir(cardsPath, 0755)
		CheckErr(err)
	}
}

func main() {
	flag.Parse()
	wg := sync.WaitGroup{}
	imageChan := make(chan ImageInfo, maxChanCount)
	client := NewClient()

	for i := 0; i < maxChanCount; i++ {
		go func() {
			for {
				imageInfo := <-imageChan
				fmt.Println(imageInfo)

				urlArr := strings.Split(imageInfo.URL, "/")
				fileName := urlArr[len(urlArr)-1]

				idolPath := path.Join(cardsPath, imageInfo.Idol)
				_, err := os.Stat(idolPath)
				if err != nil {
					err = os.Mkdir(idolPath, 0755)
					CheckErr(err)
				}

				imagePath := path.Join(idolPath, fileName)

				req, err := http.NewRequest("GET", imageInfo.URL, nil)
				CheckErr(err)

				resp, err := client.Do(req)
				CheckErr(err)

				imageBody, err := io.ReadAll(resp.Body)
				CheckErr(err)

				imageStat, err := os.Stat(imagePath)
				if err == nil {
					if int(imageStat.Size()) == len(imageBody) {
						fmt.Println("File size matched, skipping...")
						resp.Body.Close()
						wg.Done()
						continue
					}
				}

				err = os.WriteFile(imagePath, imageBody, 0644)
				CheckErr(err)

				resp.Body.Close()
				wg.Done()
			}
		}()
	}

	for i := 1; ; i++ {
		wg.Wait()
		url := fmt.Sprintf("%s%d", cardsApi, i)

		req, err := http.NewRequest("GET", url, nil)
		CheckErr(err)

		resp, err := client.Do(req)
		CheckErr(err)

		body, err := io.ReadAll(resp.Body)
		CheckErr(err)
		defer resp.Body.Close()

		doc := soup.HTMLParse(string(body))
		divs := doc.FindAll("div", "class", "top-item")
		if len(divs) == 0 {
			fmt.Println("Done!")
			break
		}

		for _, div := range divs {
			idolURL := idolURL + div.Find("a").Attrs()["data-ajax-url"]
			req, err := http.NewRequest("GET", idolURL, nil)
			CheckErr(err)

			resp, err := client.Do(req)
			CheckErr(err)

			idolBody, err := io.ReadAll(resp.Body)
			CheckErr(err)
			defer resp.Body.Close()

			idolDoc := soup.HTMLParse(string(idolBody))
			idolDiv := idolDoc.FindAll("div", "class", "top-item")
			if len(idolDiv) == 0 {
				continue
			}

			idol := idolDoc.Find("div", "data-field", "idol").Find("span", "class", "text_with_link").Text()

			cards := idolDiv[0].FindAll("a")
			for _, card := range cards {
				cardURL := "https:" + card.Attrs()["href"]
				imageChan <- ImageInfo{
					URL:  cardURL,
					Idol: idol,
				}
				wg.Add(1)
			}
		}
	}
}

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

func NewClient() (client http.Client) {
	if *httpProxy != "" {
		fmt.Println("Using http proxy...")
		proxy, err := url.Parse(*httpProxy)
		fmt.Println(proxy)
		CheckErr(err)
		client = http.Client{
			Timeout: time.Minute * 3,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxy),
			},
		}
	}

	return
}
