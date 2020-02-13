package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-pg/pg/v9"
	"github.com/go-pg/pg/v9/orm"
	"github.com/gocolly/colly/v2"
)

var DOMAIN = "https://finance.yahoo.com"

type Option struct {
	ID           int64
	ContractName string
	LastTrade    time.Time
	Strike       float64
	LastPrice    float64
	Bid          float64
	Ask          float64
	Volume       int64
	OpenInterest int64
	Ivol         string
	Symbol       string
	Expiration   time.Time
	OptionType   string
}

func WriteOptions(db *pg.DB, options *[]Option) {
	if len(*options) > 0 {
		err := db.Insert(options)
		if err != nil {
			fmt.Println(err)
		}
	}

}

func ReadInput(symbolFileLocation string) []string {
	// Read from file
	b, err := ioutil.ReadFile(symbolFileLocation) // just pass the file name
	if err != nil {
		fmt.Print(err)
	}
	str := string(b) // convert content to a 'string'
	// split each row
	rows := strings.Split(str, "\n")
	return rows
}

func ParseContractExpirations(contractCollector *colly.Collector, chainColelctor *colly.Collector) {
	contractCollector.OnHTML("select", func(e *colly.HTMLElement) {
		expirations := e.ChildAttrs("option", "value")
		symbolURL := e.Request.URL
		for i := range expirations {
			//https://finance.yahoo.com/quote/%5ESPX/options?date=1576195200
			chainColelctor.Visit(fmt.Sprintf("%s?date=%s", symbolURL, expirations[i]))
		}

	})
}

func ParseChain(db *pg.DB, chainCollector *colly.Collector) {

	var count = 0

	chainCollector.OnHTML("table", func(e *colly.HTMLElement) {
		options := make([]Option, 0, 2000)
		chainType := ""

		if e.Attr("class") == "calls" {
			chainType = "C"
		} else {
			chainType = "P"
		}
		//interates through calls and puts
		e.ForEach("table tbody tr", func(_ int, e *colly.HTMLElement) {

			cells := e.ChildTexts("td")
			//Modify empty stuff with nil
			//using for i so can modify stuff in memory
			for i := 0; i < len(cells); i++ {
				//Replacing -, with nil
				re := regexp.MustCompile(`^-$`)
				str := re.ReplaceAllString(cells[i], "")
				//Removing coma to parse int's later
				str = strings.Replace(str, ",", "", 10)
				cells[i] = str

			}
			option := Option{
				ContractName: cells[0],
				Ivol:         cells[10],
				OptionType:   chainType,
			}

			//hello go...
			dateLayout := "2006-01-02 3:04PM MST"
			lastTrade, _ := time.Parse(dateLayout, cells[1])
			option.LastTrade = lastTrade

			strike, _ := strconv.ParseFloat(cells[2], 64)
			option.Strike = strike

			lastPrice, _ := strconv.ParseFloat(cells[3], 64)
			option.LastPrice = lastPrice

			bid, _ := strconv.ParseFloat(cells[4], 64)
			option.Bid = bid

			ask, _ := strconv.ParseFloat(cells[5], 64)
			option.Ask = ask

			volume, _ := strconv.ParseInt(cells[8], 10, 64)
			option.Volume = volume

			openInterest, _ := strconv.ParseInt(cells[9], 10, 64)
			option.OpenInterest = openInterest

			//find all where ticker symbols is capital A-Z length 1 or greater
			contractSymbolRe := regexp.MustCompile("^[A-Z]{1,}")
			option.Symbol = contractSymbolRe.FindString(option.ContractName)

			//find an expiration date afteer symbol fromated as YYMMDD
			expirationDateRe := regexp.MustCompile(`[\d]{1,6}`)
			expirationDate := expirationDateRe.FindString(option.ContractName)
			expirationDateLayout := "060102"

			expiration, _ := time.Parse(expirationDateLayout, expirationDate)
			option.Expiration = expiration
			options = append(options, option)

			count++
		})

		WriteOptions(db, &options)
		options = options[:len(options)-1]
	})

	chainCollector.OnScraped(func(r *colly.Response) {
		fmt.Println("Total", count)
	})

}

func createSchema(db *pg.DB) error {
	err := db.CreateTable((*Option)(nil), &orm.CreateTableOptions{
		IfNotExists: true,
	})
	if err != nil {
		return err
	}
	db.Exec(`
	CREATE UNIQUE INDEX options_ix ON options(contract_name text_ops,last_trade timestamptz_ops);
	`)
	return nil
}

func main() {
	dbAddr := ""
	if os.Getenv("PG_ADDR") != "" {
		dbAddr = os.Getenv("PG_ADDR")
	}

	db := pg.Connect(&pg.Options{
		Addr: dbAddr,
		User: "postgres",
	})
	defer db.Close()

	err := createSchema(db)
	if err != nil {
		panic(err)
	}

	rows := ReadInput("./symbols.txt")

	fmt.Println("Got")
	fmt.Println("---------")
	fmt.Println(len(rows))
	fmt.Println("symbols")

	// Some hints on transport tweaks for scraping
	defaultRoundTripper := http.DefaultTransport
	defaultRoundTripperPointer := defaultRoundTripper.(*http.Transport)
	t := *defaultRoundTripperPointer // deref to get copy
	t.MaxIdleConns = 0
	t.MaxIdleConnsPerHost = 10000

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36"),
		colly.Async(true),
		// colly.CacheDir("./cache"),
	)

	c.WithTransport(&t)

	//TODO: Maybe add request proxy for large number of symbols

	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 50})

	d := c.Clone()

	ParseContractExpirations(c, d)
	ParseChain(db, d)

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r.StatusCode, "\nError:", err)
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	// Set error handler
	d.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "failed with response:", r.StatusCode, "\nError:", err)
	})

	d.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	for _, row := range rows {
		//https://finance.yahoo.com/quote/%5ESPX/options
		str := fmt.Sprintf("%s/quote/%s/options", DOMAIN, strings.TrimSpace(row))
		if len(str) > 1 {
			c.Visit(str)
		}
	}
	c.Wait()
	d.Wait()

}
