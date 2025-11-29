package tasks

import (
	"fmt"
	"time"

	"github.com/barweiss/go-tuple"
	"github.com/sracha4355/psp/src/entities"
	"github.com/sracha4355/psp/src/utils"
	"github.com/sracha4355/psp/src/webdriver"
	"github.com/tebeka/selenium"
)

// ---- Task Queue setup for a worker pool ----//
type Task interface {
	Run() error
}

type TaskQueue struct {
	Tasks    chan Task
	NumTasks int
}

func (q *TaskQueue) New(
	__tasks__ []Task,
) {
	q.NumTasks = len(__tasks__)
	q.Tasks = make(chan Task, q.NumTasks)
	for _, task := range __tasks__ {
		q.Tasks <- task
	}
}

// ---- specific implementation for scraping task ----//
type ScrapeLinkTask struct {
	port          int
	pagesToScrape tuple.T2[int, int]
	errorChannel  chan<- error
	linkChannel   chan<- string
}

func (task *ScrapeLinkTask) New(
	pagesToScrape tuple.T2[int, int],
	inputChannel chan<- string,
	errorChannel chan<- error,
	port int,
) {
	task.pagesToScrape = pagesToScrape
	task.linkChannel = inputChannel
	task.errorChannel = errorChannel
	task.port = port
}

func (task *ScrapeLinkTask) Run() error {
	wd, selServ, err := webdriver.StartChromeDriver(utils.CHROME_DRIVER_PATH, task.port)
	if err != nil {
		utils.TrySendError(task.errorChannel, err, true)
		return err
	}
	defer func() {
		wd.Close()
		selServ.Stop()
	}()
	const SELECTOR string = "tbody tr td:last-child a"
	firstPage, lastPage := task.pagesToScrape.V1, task.pagesToScrape.V2
	for page := firstPage; page <= lastPage; page++ {
		URL := webdriver.TradePageLink(page)
		err := __scrape_link_task_run__(wd, URL, SELECTOR, task.linkChannel)
		if err != nil {
			scrapingFailure := fmt.Errorf("failed scraping page %d: %w", page, err)
			task.errorChannel <- scrapingFailure
			return scrapingFailure
		}
	}
	return nil
}

func __scrape_link_task_run__(
	wd selenium.WebDriver,
	URL string,
	selector string,
	linkChannel chan<- string,
) error {
	if err := wd.Get(URL); err != nil {
		return fmt.Errorf("could not reach %s", URL)
	}
	// Wait for site's JS to load dynamically rendered elements
	utils.WaitXSeconds(2)
	elements, err := wd.FindElements(selenium.ByCSSSelector, selector)
	if err != nil {
		return fmt.Errorf(
			"could not find elements by selector %s on page %s",
			selector,
			URL,
		)
	}
	//---- each page will have atleast 12 links
	//---- extract links
	links := make([]string, 0, 12)
	for _, element := range elements {
		link, err := element.GetAttribute("href")
		if err != nil {
			return fmt.Errorf(
				"could not find element by selector %s on page %s",
				selector,
				URL,
			)
		}
		links = append(links, link)
	}

	//---- Scrape the trade details for each page
	for _, link := range links {
		link := fmt.Sprintf("%s%s\n", utils.URL, link)
		tradeInfo, err := __scrape_trade_details_run__(wd, link)
		if err != nil {
			return fmt.Errorf("could not scrape trade details at %s", link)
		}
		linkChannel <- tradeInfo
		utils.WaitXSeconds(3)
	}
	return nil
}

func __scrape_trade_details_run__(
	wd selenium.WebDriver,
	link string,
) (string, error) {
	if err := wd.Get(link); err != nil {
		return "", fmt.Errorf("could not reach %s", link)
	}
	// ---- wait until more info button is available
	for {
		elems, err := wd.FindElements(selenium.ByCSSSelector, entities.MORE_INFO_SELECTOR)
		if err == nil && len(elems) > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	//---- extracts the attributes of the trade 
	helper := func(selector string, link string) (string, error) {
		element, err := wd.FindElement(selenium.ByCSSSelector, selector)
		if err != nil {
			return "", fmt.Errorf(
				"could not find element by selector %s on page %s",
				entities.PARTY_SELECTOR,
				link,
			)
		}
		text, err := element.Text()
		if err != nil {
			return "", fmt.Errorf(
				"could not find extract text by selector %s on page %s",
				entities.PARTY_SELECTOR,
				link,
			)
		}
		return text, nil
	}


	moreInfoBtn, err := wd.FindElement(selenium.ByCSSSelector, entities.MORE_INFO_SELECTOR)
	if err != nil {
		return "", fmt.Errorf(
			"could not get more info by selector %s on page %s",
			entities.PARTY_SELECTOR,
			link,
		)
	}

	if err := moreInfoBtn.Click(); err != nil {
		return "", fmt.Errorf(
			"could not click more svg info by selector %s on page %s",
			entities.PARTY_SELECTOR,
			link,
		)
	}

	party, _ := helper(entities.PARTY_SELECTOR, link)
	chamber, _ := helper(entities.CHAMBER_SELECTOR, link)
	state, _ := helper(entities.STATE_SELECTOR, link)
	politicianName, _ := helper(entities.POLICITIAN_NAME_SELECTOR, link)
	tickerIssuer, _ := helper(entities.TICKER_ISSUER_SELECTOR, link)
	issuerName, _ := helper(entities.ISSUER_NAME_SELECTOR, link)
	tradedDate, _ := helper(entities.TRADED_DATE_SELECTOR, link)
	publishedDate, _ := helper(entities.PUBLISHED_DATE_SELECTOR, link)
	stockOwner, _ := helper(entities.STOCK_OWNER_SELECTOR, link)
	assetType, _ := helper(entities.ASSET_TYPE_SELECTOR, link)
	price, _ := helper(entities.PRICE_SELECTOR, link)
	shares, _ := helper(entities.SHARES_AMOUNT_SELECTOR, link)
	filledDate, _ := helper(entities.FILLED_DATE_SELECTOR, link)
	reportingGap, _ := helper(entities.REPORTING_GAP_SELECTOR, link)
	tradeValue, _ := helper(entities.TRADE_VALUE_SELECTOR, link)
	transcationType, _ := helper(entities.TRANSACTION_TYPE_SELECTOR, link)

	var policitianTrade entities.PoliticianTrade
	policitianTrade.Party = party
	policitianTrade.PoliticianType = chamber
	policitianTrade.State = state
	policitianTrade.Politician = politicianName
	policitianTrade.IssuerTicker = tickerIssuer
	policitianTrade.Issuer = issuerName
	policitianTrade.TradedDate = tradedDate
	policitianTrade.PublishedDate = publishedDate
	policitianTrade.Owner = stockOwner
	policitianTrade.AssetType = assetType
	policitianTrade.SharePrice = price
	policitianTrade.ShareAmount = shares
	policitianTrade.TradeValue = tradeValue
	policitianTrade.FilingDate = filledDate
	policitianTrade.ReportingGap = reportingGap
	policitianTrade.TranscationType = transcationType
	return policitianTrade.Stringify() + "\n", nil
}
