package main

import (
	b64 "encoding/base64"
	"fmt"
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

	//set Credentials
	if a.Config.WebServer.Credential != "" {
		a.webServerCred = b64.StdEncoding.EncodeToString([]byte(a.Config.WebServer.Credential))
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

func (a *App) Start() error {
	a.Log.Infof("starting http server: %s", a.Config.WebServer.Host)

	mux := http.NewServeMux()
	mux.HandleFunc("/query", http.HandlerFunc(a.NewQuery))

	var (
		handler_timeout int = a.Config.WebServer.HandlerTimeout
		write_timeout   int = a.Config.WebServer.WriteTimeout
		read_timeout    int = a.Config.WebServer.ReadTimeout
		idle_timeout    int = a.Config.WebServer.IdleTimeout
	)
	if handler_timeout == 0 {
		handler_timeout = HANDLER_TIMEOUT_MS
	}
	if read_timeout == 0 {
		read_timeout = READ_TIMEOUT_MS
	}
	if write_timeout == 0 {
		write_timeout = WRITE_TIMEOUT_MS
	}
	if idle_timeout == 0 {
		idle_timeout = IDLE_TIMEOUT_MS
	}
	a.WebServer = &http.Server{Addr: a.Config.WebServer.Host}
	a.WebServer.Handler = http.TimeoutHandler(mux, time.Duration(handler_timeout)*time.Millisecond, HANDLER_TIMEOUT_TEXT)
	a.WebServer.IdleTimeout = time.Duration(idle_timeout) * time.Millisecond
	a.WebServer.ReadTimeout = time.Duration(read_timeout) * time.Millisecond
	a.WebServer.WriteTimeout = time.Duration(write_timeout) * time.Millisecond

	if err := a.WebServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		a.Log.Errorf("http.ListenAndServe failed: %v", err)
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
