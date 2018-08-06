package main

import (
	"flag"
	"fmt"
	"context"
	"time"
	"net/http"
	"io/ioutil"
	"mvdan.cc/xurls"
)

func main() {
	var (
		baseUrl    = flag.String("url", "https://pastebin.com/raw/P4RekV46", "Thr starting url")
		maxWorkers = flag.Int("workers", 3, "Max. number of parallel running crawlers")
		timeout    = flag.Duration("timeout", 5000, "Milliseconds to wait per worker for the request and handling")
		depth      = flag.Int("depth", 3, "Max. layers to go in the 'tree'")
		verbose    = flag.Bool("v", true, "Output live info")
	)
	flag.Parse()

	fmt.Println(*baseUrl)
	fmt.Println(*maxWorkers)
	fmt.Println(*timeout)
	fmt.Println(*depth)

	if len(*baseUrl) == 0 {
		fmt.Println("You have to pass at least one base url.")
		return
	}
	if *maxWorkers < 1 {
		fmt.Println("Max. number of parallel running crawlers must be at least 1")
		return
	}

	// url collector channel
	urlCollector := make(chan *NewUrls, 50)
	defer close(urlCollector)

	// worker
	ctx := context.WithValue(context.Background(), "verbose", *verbose)
	ctx, cancel := context.WithTimeout(ctx, *timeout*time.Millisecond)
	defer cancel()
	workerFn := func(url string) {
		urls, err := Run(ctx, url)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		urlCollector <- &NewUrls{FromUrl:url,FoundUrls:urls}
	}

	// roll it
	go workerFn(*baseUrl)

	// start collector
	//urlTree := UrlTree{RootUrl: *baseUrl, SubUrls: make([]*UrlTree, 0)}
	//urlMap := make(map[string]*UrlTree)
coll:
	for {
		select {
		case <-ctx.Done():
			break
			break coll
		case newUrls := <-urlCollector:
			//from := newUrls.FromUrl
			urls := *newUrls.FoundUrls

			//urlMap[from] = urls
			for _, url := range urls {
				go workerFn(url)
			}

			fmt.Printf("GOT THEM: %d\n", len(urls))
			//break coll
		}
	}
}

type NewUrls struct {
	FromUrl string
	FoundUrls *[]string
}

type UrlTree struct {
	RootUrl string
	SubUrls []*UrlTree // urls found on the url
}

// Worker
func Run(ctx context.Context, url string) (*[]string, error) {
	workerTimestamp := time.Now()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %s", err.Error())
	}
	req = req.WithContext(ctx)

	reqTimestamp := time.Now()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %s", err.Error())
	}
	reqTime := time.Since(reqTimestamp)
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code not 200 got %d", res.StatusCode)
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read body: %s", err.Error())
	}
	bodyString := string(bodyBytes)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		matches := xurls.Strict().FindAllString(bodyString, -1)

		workerTime := time.Since(workerTimestamp)
		if ctx.Value("verbose") == true {
			fmt.Printf("Worker[%s]Request[%s] %d links found on page %s\n", workerTime.String(), reqTime.String(), len(matches), url)
		}
		return &matches, nil
	}
}
