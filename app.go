package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/gommon/log"
)

const (
	AUTH_PREF      = "basic "
	DEF_LOG_FORMAT = "${time_rfc3339_nano} ${short_file}:${line} ${level} -${message}"
	DEF_LOG_LEVEL  = "debug"

	IDLE_TIMEOUT_MS      = 5 * 1000
	READ_TIMEOUT_MS      = 10 * 1000
	WRITE_TIMEOUT_MS     = 20 * 1000
	HANDLER_TIMEOUT_MS   = 5 * 1000
	HANDLER_TIMEOUT_TEXT = "closed on timeout"

	CONF_TIME_LAYOUT = "15:04"

	API_TRY_CNT         = 5
	API_WAIT_SEC        = 3
	API_TOKEN_HEADER_ID = "api-token"
)

type App struct {
	Config    *AppConfig
	Log       *log.Logger
	WebServer *http.Server

	webServerCred string
	sqlFilter     string
}

func NewApp() *App {
	return &App{Config: &AppConfig{}}
}

func (a *App) LoadConfig(configData []byte) error {

	if err := a.Config.Load(configData); err != nil {
		return err
	}

	if err := a.initLogger(); err != nil {
		return err
	}

	//build sql filter string
	a.SetSQLFilter()

	return nil
}

func (a *App) initLogger() error {
	if a.Config.LogLevel == "" {
		a.Config.LogLevel = "debug"
	}
	var lvl log.Lvl
	switch a.Config.LogLevel {
	case "debug":
		lvl = log.DEBUG
		break
	case "info":
		lvl = log.INFO
		break
	case "warn":
		lvl = log.WARN
		break
	case "error":
		lvl = log.ERROR
		break
	default:
		lvl = log.INFO
	}
	//init logger
	a.Log = log.New("-")
	a.Log.SetHeader(DEF_LOG_FORMAT)
	a.Log.SetLevel(lvl)

	if a.Config.LogTo == "" || a.Config.LogTo == "stdout" {
		a.Log.SetOutput(os.Stdout)
	} else {
		log_file := ""
		if a.Config.LogFile == "" {
			log_file = "log.txt"
		} else {
			log_file = a.Config.LogFile
		}
		f, err := os.OpenFile(log_file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		a.Log.SetOutput(f)
	}

	a.Log.Infof("logging level set to: %s", a.Config.LogLevel)

	return nil
}

func (a *App) NextActDate(now time.Time, actTime string) (time.Time, error) {
	act_time, err := time.Parse(CONF_TIME_LAYOUT, actTime)
	if err != nil {
		return time.Time{}, err
	}
	act_dt := time.Date(now.Year(), now.Month(), now.Day(), act_time.Hour(), act_time.Minute(), 0, 0, now.Location())
	if now.Hour()*60+now.Minute() >= act_time.Hour()*60+act_time.Minute() {
		act_dt = act_dt.Add(24 * time.Hour)
	}
	// fmt.Println("actTime:", actTime, "act_dt:", act_dt)
	return act_dt, nil
}

type ReportPeriodDate time.Time

const ReportPeriodLoyout = "2006-01-02"

func (d *ReportPeriodDate) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), "\"")
	if s == "null" {
		*d = ReportPeriodDate(time.Time{})
		return nil
	}
	t, err := time.Parse(ReportPeriodLoyout, s)
	if err != nil {
		return err
	}
	*d = ReportPeriodDate(t)
	return nil
}

type LastSaleDateResponse struct {
	LastSaleDate ReportPeriodDate `json:"last_sale_date"`
}

func (a *App) FetchReportPerod(url string, apiKey string) (time.Time, time.Time, error) {
	// d1, err := time.Parse(ReportPeriodLoyout, "2024-07-01")
	// if err != nil {
	// 	return time.Time{}, time.Time{}, err
	// }
	// d2, err := time.Parse(ReportPeriodLoyout, "2024-07-01")
	// if err != nil {
	// 	return time.Time{}, time.Time{}, err
	// }
	// return d1, d2, nil
	tries_for_query := API_TRY_CNT
	for tries_for_query > 0 {
		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			a.Log.Errorf("http.NewRequest() failed: %v", err)
			time.Sleep(time.Duration(API_WAIT_SEC) * time.Second)
			tries_for_query--
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(API_TOKEN_HEADER_ID, apiKey) //add API key

		resp, err := client.Do(req)
		if err != nil {
			a.Log.Errorf("http.Do() failed: %v", err)
			time.Sleep(time.Duration(API_WAIT_SEC) * time.Second)
			tries_for_query--
			continue
		}

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			a.Log.Errorf("io.ReadAll() failed: %v", err)
			time.Sleep(time.Duration(API_WAIT_SEC) * time.Second)
			tries_for_query--
			continue
		}

		date_resp := LastSaleDateResponse{}
		if err := json.Unmarshal(body, &date_resp); err != nil {
			a.Log.Errorf("json.Unmarshal() failed to unmarshal last sale date: %v", err)
			time.Sleep(time.Duration(API_WAIT_SEC) * time.Second)
			tries_for_query--
			continue
		}
		//set toDate to end of day
		dt_from := time.Time(date_resp.LastSaleDate)
		n := time.Now()
		dt_to := time.Date(n.Year(), n.Month(), n.Day(), 23, 59, 59, 999, n.Location())

		return dt_from, dt_to, nil
	}
	return time.Time{}, time.Time{}, fmt.Errorf("error fetching report period")
}

func (a *App) SendData(rkData []RKDate, url string, apiKey string) error {
	//marshal data
	rk_data_b, err := json.Marshal(rkData)
	if err != nil {
		return err
	}

	//wrapper
	rk_data_b_wr := []byte(`{"data":`)
	rk_data_b_wr = append(rk_data_b_wr, rk_data_b...)
	rk_data_b_wr = append(rk_data_b_wr, []byte(`}`)...)

	//send
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(rk_data_b_wr))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(API_TOKEN_HEADER_ID, apiKey) //add API key

	tries_for_query := API_TRY_CNT
	for tries_for_query > 0 {
		resp, err := client.Do(req)
		if err != nil {
			a.Log.Errorf("client.Do() failed: %v", err)
			time.Sleep(time.Duration(API_WAIT_SEC) * time.Second)
			tries_for_query--
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				a.Log.Errorf("io.ReadAll() failed to read body: %v", err)
				time.Sleep(time.Duration(API_WAIT_SEC) * time.Second)
				tries_for_query--
				continue
			}
			a.Log.Errorf("API send data response status code: %d, body: %s", resp.StatusCode, string(body))
			a.Log.Errorf("request url: %s, request body: %s", url, string(rk_data_b_wr))
			time.Sleep(time.Duration(API_WAIT_SEC) * time.Second)
			tries_for_query--
			continue
		}
		return nil
	}
	return fmt.Errorf("error sending data")
}

func ensureSlash(url string) string {
	url_l := len(url)
	url_cor := url
	if url[url_l-1:url_l] != "/" {
		url_cor += "/"
	}
	return url_cor
}

// main loop
func (a *App) Start() error {
	//checking
	if a.Config.APIUrl == "" {
		return fmt.Errorf("API url not set")
	}
	if a.Config.APIKey == "" {
		return fmt.Errorf("APIKey not set")
	}
	if a.Config.ScID == "" {
		return fmt.Errorf("scID not set")
	}
	if a.Config.SaleLocationID == "" {
		return fmt.Errorf("saleLocationID not set")
	}

	url := ensureSlash(a.Config.APIUrl)
	url = strings.ReplaceAll(url, "{{scID}}", a.Config.ScID)
	url = strings.ReplaceAll(url, "{{saleLocationID}}", a.Config.SaleLocationID)

	if a.Config.APICmdGetPeriod == "" {
		return fmt.Errorf("API cmdGetPeriod not set")
	}
	cmd_period := ensureSlash(a.Config.APICmdGetPeriod)

	if a.Config.APICmdPutData == "" {
		return fmt.Errorf("API cmdPutPeriod not set")
	}
	cmd_data := ensureSlash(a.Config.APICmdPutData)

	rep_period_url := fmt.Sprintf("%s%s", url, cmd_period)
	send_data_url := fmt.Sprintf("%s%s", url, cmd_data)
	a.Log.Debugf("get date url: %s", rep_period_url)
	a.Log.Debugf("put data url: %s", send_data_url)
	first_query := true

	//main wait loop. On first start send query anyway.
	for {
		if !first_query {
			act_dt, err := a.NextActDate(time.Now(), a.Config.ActivationTime)
			if err != nil {
				a.Log.Errorf("NextActivationTime() failed: %v", err)
				return err
			}
			dur := act_dt.Sub(time.Now())
			a.Log.Debugf("Next activation time: %v, sleep interval: %v", act_dt, dur)
			time.Sleep(dur)
		}

		//retrieve period for this client
		dt_from, dt_to, err := a.FetchReportPerod(rep_period_url, a.Config.APIKey)
		if err != nil {
			if first_query {
				return err
			}
			a.Log.Errorf("FetchReportPerod() failed: %v", err)
			continue
		}

		//fetch data from MS server till no more is available
		from := 0
		count := DEF_PARAM_COUNT
		for {
			a.Log.Debugf("Fetching data for period: %s %s", dt_from.Format(PARAM_DATE_LAYOUT), dt_to.Format(PARAM_DATE_LAYOUT))
			rk_data, err := a.FetchRKData(context.Background(), a.Config.MSCon, from, count, dt_from, dt_to)
			if err != nil {
				if first_query {
					return err
				}
				a.Log.Errorf("FetchRKData() failed: %v", err)
				//response with error?
				break
			}

			a.Log.Debugf("got records: %d", len(rk_data))
			if len(rk_data) == 0 {
				break //no data
			}

			if err := a.SendData(rk_data, send_data_url, a.Config.APIKey); err != nil {
				if first_query {
					return err
				}
				a.Log.Errorf("SendData() failed: %v", err)
				break
			}
			from += count
		}
		first_query = false
	}
	return nil
}

// SetSQLFilter builds sql filter string.
func (a *App) SetSQLFilter() {
	a.sqlFilter = ""

	var cond strings.Builder
	if len(a.Config.Restaurants) > 0 {
		// add restaurant condition
		var rest_cond strings.Builder
		for _, rest := range a.Config.Restaurants {
			if rest_cond.Len() > 0 {
				rest_cond.WriteString(" OR ")
			}
			rest_cond.WriteString(fmt.Sprintf("RESTAURANTS.NAME = '%s'", rest))
		}
		if cond.Len() > 0 {
			cond.WriteString(" AND ")
		}
		cond.WriteString("(")
		cond.WriteString(rest_cond.String())
		cond.WriteString(")")
	}
	if len(a.Config.CashGroups) > 0 {
		// add cash group condition
		var gr_cond strings.Builder
		for _, gr := range a.Config.CashGroups {
			if gr_cond.Len() > 0 {
				gr_cond.WriteString(" OR ")
			}
			gr_cond.WriteString(fmt.Sprintf("CASHGROUPS.NAME = '%s'", gr))
		}
		if cond.Len() > 0 {
			cond.WriteString(" AND ")
		}
		cond.WriteString("(")
		cond.WriteString(gr_cond.String())
		cond.WriteString(")")
	}
	if cond.Len() > 0 {
		a.sqlFilter = cond.String()
	}
}
