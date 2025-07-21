package main

import (
	"encoding/json"
	"os"
	"log/slog"
	"fmt"
	"net/http"
	"time"
	"github.com/spf13/viper"
	"bytes"
)

//---------------- STRUCTS -----------------
type LOWESTPrice struct {
	Price 	float64
	Start	time.Time
	End 	time.Time
	CurrentPrice float64
}

type EVCCState struct {
	Result struct {		
		TariffGrid				float64 `json:"tariffGrid"`
		Forecast                struct {
			Grid []struct {
				Start time.Time `json:"start"`
				End   time.Time `json:"end"`
				Price float64   `json:"value"`
			} `json:"grid"`
		} `json:"forecast"`
	}
}

//---------------- FUNCTIONS -----------------
func getEVCCState(url string, start int, end int)(LOWESTPrice){
	slog.Info(fmt.Sprintf("Checking for lowest Price between %d:00 and %d:00", start, end))
	evccURL := fmt.Sprintf("http://%s/api/state", url)

	res, err := http.Get(evccURL)
    if err != nil {
        slog.Error("error making http request","ERR", err)
        os.Exit(1)
    }
    defer res.Body.Close()

	var evccResp EVCCState
	err = json.NewDecoder(res.Body).Decode(&evccResp)
	if err != nil {
		slog.Error("error decoding data", "ERR", err)
	}

	var lowestPrice LOWESTPrice
	lowestPrice.Price = evccResp.Result.Forecast.Grid[0].Price
	lowestPrice.CurrentPrice = evccResp.Result.TariffGrid

	// Lowest Price between START and END
	for i :=start; i<=end; i++ {
		if evccResp.Result.Forecast.Grid[i].Price < lowestPrice.Price {
			lowestPrice.Price = evccResp.Result.Forecast.Grid[i].Price
			lowestPrice.Start = evccResp.Result.Forecast.Grid[i].Start
			lowestPrice.End = evccResp.Result.Forecast.Grid[i].End
		}else if evccResp.Result.Forecast.Grid[i].Price == lowestPrice.Price { 
			lowestPrice.End = evccResp.Result.Forecast.Grid[i].End
		}
	}
	slog.Info(fmt.Sprintf("Current Price: %.3f Euro/kWh", lowestPrice.CurrentPrice))
	slog.Info(fmt.Sprintf("Lowest Price: %.3f Euro/kWh starting at %d:00 - ending at %d:00", lowestPrice.Price, lowestPrice.Start.Hour(), lowestPrice.End.Hour()))

	return lowestPrice
}

func haUpdate(haEntitiy string, haHost string, haToken string, lowestPrice LOWESTPrice) {
		client := &http.Client{}
		payload := []byte(fmt.Sprintf(`{
			"state": "%d:00 - %d:00"
		}`,lowestPrice.Start.Hour(), lowestPrice.End.Hour()))

		url := fmt.Sprintf("http://%s/api/states/%s", haHost, haEntitiy)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
		if err != nil {
			slog.Error("ERR",err)
		}
		
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", "Bearer "+haToken)
	
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("ERR",err)
		}
		defer resp.Body.Close()
}

//---------------- MAIN -----------------
func main() {	
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("toml") // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/ttc/")   // path to look for the config file in
	viper.AddConfigPath(".")               // optionally look for config in the working directory
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}

	var evccHost = viper.GetString("evcc.host")
	var Interval = viper.GetInt("global.interval")

	fmt.Println("\n-- Tibber Tarrif Check via EVCC --\n")

	logger := slog.New(slog.NewJSONHandler(os.Stderr,nil))
	if viper.GetBool("global.debug") {
		logger = slog.New(slog.NewJSONHandler(os.Stderr,&slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	
	slog.SetDefault(logger)
	slog.Debug("Application started")

	if Interval != 0 {
		for {
			// Refresh Config from File
			err = viper.ReadInConfig()
			if err != nil { // Handle errors reading the config file
				panic(fmt.Errorf("Fatal error config file: %w \n", err))
			}
			Interval = viper.GetInt("global.interval")
			evccMorningStart := viper.GetInt("evcc.morning.start")
			evccMorningEnd := viper.GetInt("evcc.morning.end")
			evccAfternoonStart := viper.GetInt("evcc.afternoon.start")
			evccAfternoonEnd := viper.GetInt("evcc.afternoon.end")
			haHost := viper.GetString("homeassistant.host")
			haToken := viper.GetString("homeassistant.token")
			haEntitiy := viper.GetString("homeassistant.text_entityid")

			slog.Debug(fmt.Sprintf("Current Time: %s", time.Now()))
			slog.Debug("Config Setting","Interval", Interval)
			slog.Debug("Config Setting","EVCC_Host",evccHost)
			slog.Debug("Config Setting","Morning_Start",evccMorningStart)
			slog.Debug("Config Setting","Morning_End",evccMorningEnd)
			slog.Debug("Config Setting","Afternoon_Start",evccAfternoonStart)
			slog.Debug("Config Setting","Afternoon_End",evccAfternoonEnd)
			slog.Debug("Config Setting","HA Host",haHost)
			slog.Debug("Config Setting","HA Token",haToken)
			slog.Debug("Config Setting","HA EntitiyID",haEntitiy)
			
			t := time.Now() 
			var lowestPrice LOWESTPrice

			switch t.Hour(); {
				case t.Hour() >= evccMorningStart && t.Hour() < evccMorningEnd:
					// Morning Check
					slog.Debug("Performing Morning Check based on current time")
					lowestPrice = getEVCCState(evccHost, evccMorningStart, evccMorningEnd)	
				case t.Hour() >= evccAfternoonStart && t.Hour() < evccAfternoonEnd:
					// Afternnon Check
					slog.Debug("Performing Afternoon Check based on current time")
					lowestPrice = getEVCCState(evccHost, evccAfternoonStart, evccAfternoonEnd)
				default:
					slog.Info("!! OFF SHIFT !!")
			}
			
			haUpdate(haEntitiy, haHost, haToken, lowestPrice)

			// Sleep for X Seconds
			slog.Debug(fmt.Sprintf("Sleeping for %d seconds", Interval))
			time.Sleep(time.Duration(Interval) * time.Second)
			fmt.Println("")
		}
	}
}
