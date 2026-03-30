package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

type Simulation struct {
	TargetRecords    int64 `json:"target_records"`
	UniqueRecords    int64 `json:"unique_records"`
	DuplicateRecords int64 `json:"duplicate_records"`
	ProcessingTimeMs int64 `json:"processing_time_ms"`
}

type AggregatedSimulation struct {
	TargetRecords        int64        `json:"target_records"`
	Trials               int          `json:"trials"`
	MeanUnique           float64      `json:"mean_unique"`
	MinUnique            int64        `json:"min_unique"`
	MaxUnique            int64        `json:"max_unique"`
	MeanDuplicate        float64      `json:"mean_duplicate"`
	MinDuplicate         int64        `json:"min_duplicate"`
	MaxDuplicate         int64        `json:"max_duplicate"`
	MeanProcessingTimeMs float64      `json:"mean_processing_time_ms"`
	MinProcessingTimeMs  int64        `json:"min_processing_time_ms"`
	MaxProcessingTimeMs  int64        `json:"max_processing_time_ms"`
	RawResults           []Simulation `json:"raw_results,omitempty"`
}

type DigitsResult struct {
	Digits            int                    `json:"digits"`
	TotalCombinations int64                  `json:"total_combinations"`
	Results           []AggregatedSimulation `json:"results"`
}

func runSimulation(digits int, numRecords int64, rng *rand.Rand) Simulation {
	chars := int64(16)
	totalCombinations := int64(math.Pow(float64(chars), float64(digits)))

	startTime := time.Now()

	generatedSet := make(map[int64]struct{}, numRecords)

	for i := int64(0); i < numRecords; i++ {
		val := rng.Int63n(totalCombinations)
		generatedSet[val] = struct{}{}
	}

	duration := time.Since(startTime)
	uniqueCount := int64(len(generatedSet))
	duplicateCount := numRecords - uniqueCount

	return Simulation{
		TargetRecords:    numRecords,
		UniqueRecords:    uniqueCount,
		DuplicateRecords: duplicateCount,
		ProcessingTimeMs: duration.Milliseconds(),
	}
}

func runTrials(digits int, numRecords int64, trials int) AggregatedSimulation {
	ch := make(chan Simulation, trials)
	var wg sync.WaitGroup
	wg.Add(trials)

	for i := 0; i < trials; i++ {
		i := i
		go func(idx int) {
			defer wg.Done()
			seed := time.Now().UnixNano() + int64(idx)*10007
			rng := rand.New(rand.NewSource(seed))
			res := runSimulation(digits, numRecords, rng)
			ch <- res
		}(i)
	}

	wg.Wait()
	close(ch)

	raw := make([]Simulation, 0, trials)
	for s := range ch {
		raw = append(raw, s)
	}

	n := float64(len(raw))
	var sumUnique, sumDuplicate, sumTime float64
	for _, s := range raw {
		sumUnique += float64(s.UniqueRecords)
		sumDuplicate += float64(s.DuplicateRecords)
		sumTime += float64(s.ProcessingTimeMs)
	}

	meanUnique := 0.0
	meanDuplicate := 0.0
	meanTime := 0.0
	if n > 0 {
		meanUnique = sumUnique / n
		meanDuplicate = sumDuplicate / n
		meanTime = sumTime / n
	}

	var minUnique, maxUnique, minDuplicate, maxDuplicate, minTime, maxTime int64
	if len(raw) > 0 {
		minUnique = raw[0].UniqueRecords
		maxUnique = raw[0].UniqueRecords
		minDuplicate = raw[0].DuplicateRecords
		maxDuplicate = raw[0].DuplicateRecords
		minTime = raw[0].ProcessingTimeMs
		maxTime = raw[0].ProcessingTimeMs
		for _, s := range raw[1:] {
			if s.UniqueRecords < minUnique {
				minUnique = s.UniqueRecords
			}
			if s.UniqueRecords > maxUnique {
				maxUnique = s.UniqueRecords
			}
			if s.DuplicateRecords < minDuplicate {
				minDuplicate = s.DuplicateRecords
			}
			if s.DuplicateRecords > maxDuplicate {
				maxDuplicate = s.DuplicateRecords
			}
			if s.ProcessingTimeMs < minTime {
				minTime = s.ProcessingTimeMs
			}
			if s.ProcessingTimeMs > maxTime {
				maxTime = s.ProcessingTimeMs
			}
		}
	}

	return AggregatedSimulation{
		TargetRecords:        numRecords,
		Trials:               len(raw),
		MeanUnique:           meanUnique,
		MinUnique:            minUnique,
		MaxUnique:            maxUnique,
		MeanDuplicate:        meanDuplicate,
		MinDuplicate:         minDuplicate,
		MaxDuplicate:         maxDuplicate,
		MeanProcessingTimeMs: meanTime,
		MinProcessingTimeMs:  minTime,
		MaxProcessingTimeMs:  maxTime,
	}
}

func main() {
	trials := flag.Int("trials", 100, "number of parallel trials per combination")
	flag.Parse()

	digitsList := []int{7, 8, 9, 10}
	recordsList := []int64{400000, 800000, 1200000, 1600000, 2000000}

	var allResults []DigitsResult

	for _, d := range digitsList {
		chars := int64(16)
		totalCombinations := int64(math.Pow(float64(chars), float64(d)))

		dr := DigitsResult{
			Digits:            d,
			TotalCombinations: totalCombinations,
			Results:           make([]AggregatedSimulation, 0, len(recordsList)),
		}

		for _, r := range recordsList {
			agg := runTrials(d, r, *trials)
			dr.Results = append(dr.Results, agg)
		}

		allResults = append(allResults, dr)
	}

	jsonData, err := json.MarshalIndent(allResults, "", "  ")
	if err != nil {
		fmt.Printf(`{"error": "%s"}`+"\n", err.Error())
		return
	}

	fmt.Println(string(jsonData))
}
