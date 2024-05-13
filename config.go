package main

import (
	"bytes"
	"encoding/json"
)

type WebServer struct {
	Host           string `json:"host"`        //web server host:port
	IdleTimeout    int    `json:"idleTimeout"` // all parameters are in milliseconds
	ReadTimeout    int    `json:"readTimeout"`
	WriteTimeout   int    `json:"writeTimeout"`
	HandlerTimeout int    `json:"handlerTimeout"`
	Credential     string `json:"credential"` //login:password
}
type AppConfig struct {
	LogTo    string `json:"logTo"` // where to log: stdout|file, stdout is default
	LogFile  string `json:"logFile"`
	LogLevel string `json:"logLevel"` // debug|info|warn|error, info - default
	MSCon    string `json:"msCon"`    // microsoft sql connection string in format: sqlserver://username:password@host:port/instance?database=name

	Restaurants []string `json:"restaurants"` // names from 'restaurants' table or empty for all restaurants
	CashGroups  []string `json:"cashGroups"`  // names from cashgroups table or empty for all cash groups

	WebServer WebServer `json:"webServer"`
}

func (c *AppConfig) Load(configData []byte) error {
	configData = bytes.TrimPrefix(configData, []byte("\xef\xbb\xbf"))
	if err := json.Unmarshal([]byte(configData), &c); err != nil {
		return err
	}

	return nil
}
