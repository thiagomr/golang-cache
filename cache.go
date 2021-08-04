package sample1

import (
	"fmt"
	"sync"
	"time"
)

// PriceService is a service that we can use to get prices for the items
// Calls to this service are expensive (they take time)
type PriceService interface {
	GetPriceFor(itemCode string) (float64, error)
}

// TransparentCache is a cache that wraps the actual service
// The cache will remember prices we ask for, so that we don't have to wait on every call
// Cache should only return a price if it is not older than "maxAge", so that we don't get stale prices
type TransparentCache struct {
	actualPriceService PriceService
	maxAge             time.Duration
	prices             map[string]float64
	pricesAge          map[string]time.Time
}

func NewTransparentCache(actualPriceService PriceService, maxAge time.Duration) *TransparentCache {
	return &TransparentCache{
		actualPriceService: actualPriceService,
		maxAge:             maxAge,
		prices:             map[string]float64{},
		pricesAge:          map[string]time.Time{},
	}
}

// GetPriceFor gets the price for the item, either from the cache or the actual service if it was not cached or too old
func (c *TransparentCache) GetPriceFor(itemCode string) (float64, error) {
	price, priceOk := c.prices[itemCode]
	priceAge, priceAgeOk := c.pricesAge[itemCode]

	if priceOk && priceAgeOk && time.Since(priceAge) < c.maxAge {
		return price, nil
	}

	price, err := c.actualPriceService.GetPriceFor(itemCode)

	if err != nil {
		return 0, fmt.Errorf("getting price from service : %v", err.Error())
	}

	c.prices[itemCode] = price
	c.pricesAge[itemCode] = time.Now()

	return price, nil
}

// GetPricesFor gets the prices for several items at once, some might be found in the cache, others might not
// If any of the operations returns an error, it should return an error as well
func (c *TransparentCache) GetPricesFor(itemCodes ...string) ([]float64, error) {
	nitems := len(itemCodes)
	results := []float64{}

	cresults := make(chan float64, nitems)
	cfinished := make(chan bool, 1)
	cerr := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(nitems)

	for _, itemCode := range itemCodes {
		go func(code string) {
			defer wg.Done()
			price, err := c.GetPriceFor(code)

			if err != nil {
				cerr <- err
			}

			cresults <- price
		}(itemCode)
	}

	go func() {
		wg.Wait()
		close(cfinished)
	}()

	select {
	case <-cfinished:
		for i := 0; i < nitems; i++ {
			results = append(results, <-cresults)
		}

		return results, nil
	case err := <-cerr:
		{
			fmt.Println("get prices error", err)
			return []float64{}, nil
		}
	}
}
