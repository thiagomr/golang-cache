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
	mutex              sync.Mutex
}

func NewTransparentCache(actualPriceService PriceService, maxAge time.Duration) *TransparentCache {
	return &TransparentCache{
		actualPriceService: actualPriceService,
		maxAge:             maxAge,
		prices:             map[string]float64{},
		pricesAge:          map[string]time.Time{},
		mutex:              sync.Mutex{},
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

	c.mutex.Lock()
	c.prices[itemCode] = price
	c.pricesAge[itemCode] = time.Now()
	c.mutex.Unlock()

	return price, nil
}

// GetPricesFor gets the prices for several items at once, some might be found in the cache, others might not
// If any of the operations returns an error, it should return an error as well
func (c *TransparentCache) GetPricesFor(itemCodes ...string) ([]float64, error) {
	nItems := len(itemCodes)
	results := []float64{}

	cResults := make(chan float64, nItems)
	cFinished := make(chan bool, 1)
	cErr := make(chan error, 1)

	var wg sync.WaitGroup
	wg.Add(nItems)

	for _, itemCode := range itemCodes {
		go func(code string) {
			defer wg.Done()
			price, err := c.GetPriceFor(code)

			if err != nil {
				cErr <- err
			}

			cResults <- price
		}(itemCode)
	}

	go func() {
		wg.Wait()
		close(cFinished)
	}()

	select {
	case <-cFinished:
		for i := 0; i < nItems; i++ {
			results = append(results, <-cResults)
		}

		return results, nil
	case <-cErr:
		{
			return []float64{}, nil
		}
	}
}
