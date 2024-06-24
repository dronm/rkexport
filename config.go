package main

import (
	"bytes"
	"encoding/json"
)

type AppConfig struct {
	LogTo    string `json:"logTo"` // where to log: stdout|file, stdout is default
	LogFile  string `json:"logFile"`
	LogLevel string `json:"logLevel"` // debug|info|warn|error, info - default
	MSCon    string `json:"msCon"`    // microsoft sql connection string in format: sqlserver://username:password@host:port/instance?database=name

	Restaurants []string `json:"restaurants"` // names from 'restaurants' table or empty for all restaurants
	CashGroups  []string `json:"cashGroups"`  // names from cashgroups table or empty for all cash groups

	APIUrl          string `json:"apiUrl"`
	APICmdGetPeriod string `json:"apiCmdGetPeriod"`
	APICmdPutData   string `json:"apiCmdPutData"`
	APIKey          string `json:"apiKey"`
	ActivationTime  string `json:"activationTime"` //time in format 00:00
	TradecenterID   string `json:"tradecenterID"`
}

func (c *AppConfig) Load(configData []byte) error {
	configData = bytes.TrimPrefix(configData, []byte("\xef\xbb\xbf"))
	if err := json.Unmarshal([]byte(configData), &c); err != nil {
		return err
	}

	return nil
}
