package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"io"
	"encoding/csv"
	"time"
)

type Race struct {
	horseId     int
	entryFee    float64
	finishTimes []float64
}

var races []Race

type racesByHorseID []Race

func (r racesByHorseID) Len() int {
	return len(r)
}

func (r racesByHorseID) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r racesByHorseID) Less(i, j int) bool {
	return r[i].horseId < r[j].horseId
}

// Modify the Race struct to initialize the finishTimes slice with a length of 0.
func NewRace(horseId int, entryFee float64) *Race {
	return &Race{
		horseId:     horseId,
		entryFee:    entryFee,
		finishTimes: make([]float64, 0),
	}
}


func readRacesCSV(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // to allow variable number of fields
	reader.TrimLeadingSpace = true

	var header []string
	for i := 0; ; i++ {
		line, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read file: %v", err)
		}

		if i == 0 {
			header = line
			continue
		}

		record := make(map[string]string)
		for i, field := range line {
			record[header[i]] = field
		}

		horseID, err := strconv.Atoi(record["horseId"])
		if err != nil {
			return fmt.Errorf("failed to parse horseId: %v", err)
		}

		entryFee, err := strconv.ParseFloat(record["entryFee"], 64)
		if err != nil {
			return fmt.Errorf("failed to parse entryFee: %v", err)
		}

		finishTime, err := strconv.ParseFloat(record["finishTime"], 64)
		if err != nil {
			return fmt.Errorf("failed to parse finishTime: %v", err)
		}

		var race *Race
		for i := range races {
			if races[i].horseId == horseID {
				race = &races[i]
				break
			}
		}

		if race == nil {
			// Use NewRace function to create a new Race instance with empty finishTimes slice.
			race = NewRace(horseID, entryFee)
			races = append(races, *race)
		}

		race.finishTimes = append(race.finishTimes, finishTime)
		
	}

	for i := range races {
		sort.Float64s(races[i].finishTimes)
	}

	return nil
}


func calculateMeanSpeed(horseSpeeds []float64, numSamples int) float64 {
	if len(horseSpeeds) == 0 {
		return 0
	}

	// Compute the CDF of the horse speeds.
	cdf := make([]float64, len(horseSpeeds))
	sum := 0.0
	for i := 0; i < len(horseSpeeds); i++ {
		sum += horseSpeeds[i]
		cdf[i] = sum
	}
	for i := 0; i < len(horseSpeeds); i++ {
		cdf[i] /= sum
	}

	rand.Seed(time.Now().UnixNano())

	totalSpeed := 0.0

	for i := 0; i < numSamples; i++ {
		r := rand.Float64()
		// Use binary search to find the index of the horse with the speed
		// corresponding to the random sample from the CDF.
		index := sort.Search(len(cdf), func(j int) bool {
			return cdf[j] >= r
		})
		speed := horseSpeeds[index]
		totalSpeed += speed
	}

	return totalSpeed / float64(numSamples)
}

func getFasterHorse(race Race) (int, float64) {
	if len(race.finishTimes) == 0 {
		return 0, 0
	}

	fastest := race.finishTimes[0]
	fasterHorse := 1

	for i := 1; i < len(race.finishTimes); i++ {
		speed := 1000 / race.finishTimes[i]
		if speed > 1000/fastest {
			fastest = race.finishTimes[i]
			fasterHorse = i + 1
		}
	}

	return fasterHorse, fastest
}


func getSlowerHorse(race Race) (int, float64) {
	if len(race.finishTimes) == 0 {
		return 0, 0
	}

	slowest := race.finishTimes[0]
	slowerHorse := 1

	for i := 1; i < len(race.finishTimes); i++ {
		speed := 1000 / race.finishTimes[i]
		if speed < 1000/slowest {
			slowest = race.finishTimes[i]
			slowerHorse = i + 1
		}
	}

	return slowerHorse, slowest
}

func compareMeanSpeed(w http.ResponseWriter, r *http.Request) {
	// Parse horse IDs from GET parameters
	h1, err := strconv.Atoi(r.URL.Query().Get("horse1"))
	if err != nil {
		http.Error(w, "Invalid horse1 parameter", http.StatusBadRequest)
		return
	}
	h2, err := strconv.Atoi(r.URL.Query().Get("horse2"))
	if err != nil {
		http.Error(w, "Invalid horse2 parameter", http.StatusBadRequest)
		return
	}

	// Find races for each horse
	h1Races := racesByHorseID(races)[h1-1]
	h2Races := racesByHorseID(races)[h2-1]

	// Calculate mean speed for each horse
	numSamples := 1000
	h1MeanSpeed := calculateMeanSpeed(h1Races.finishTimes, numSamples)
	h2MeanSpeed := calculateMeanSpeed(h2Races.finishTimes, numSamples)

	// Determine which horse is faster
	var fasterHorse int
	var slowerHorse int
	var fasterSpeed float64
	var slowerSpeed float64
	if h1MeanSpeed > h2MeanSpeed {
		fasterHorse = h1
		slowerHorse = h2
		fasterSpeed = h1MeanSpeed
		slowerSpeed = h2MeanSpeed
	} else {
		fasterHorse = h2
		slowerHorse = h1
		fasterSpeed = h2MeanSpeed
		slowerSpeed = h1MeanSpeed
	}

	// Construct response JSON
	response := fmt.Sprintf(`{"fasterHorse": %d, "slowerHorse": %d, "fasterSpeed": %f, "slowerSpeed": %f}`,
		fasterHorse, slowerHorse, fasterSpeed, slowerSpeed)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(response))
}


func main() {
	err := readRacesCSV("races-for-tay.csv")
	if err != nil {
		panic(err)
	}

	err = readRacesCSV("races-for-tay.csv")
	if err != nil {
		fmt.Printf("Error reading races CSV: %v", err)
		return
	}

	http.HandleFunc("/compare", compareMeanSpeed)

	fmt.Println("Starting server...")
	http.ListenAndServe(":8080", nil)
}
