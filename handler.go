package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

const SQL_FILE_NAME = "msQuery.sql"
const PARAM_DATE_LAYOUT = "2006-01-02T15:04:05.999"

const (
	DEF_PARAM_FROM  = 0
	DEF_PARAM_COUNT = 100
)

// RKDate is retrieved from MSSQL
type RKDate struct {
	CheckOpen       time.Time `json:"checkOpen"`       // • Дата/время открытия/закрытия заказа
	CheckClose      time.Time `json:"checkClose"`      // • Дата/время открытия/закрытия заказа
	VisitStartTime  time.Time `json:"visitStartTime"`  // • Дата/время формирования пречека
	OrderNum        string    `json:"orderNum"`        // • Номер заказа
	FiscDocNum      string    `json:"fiscDocNum"`      // • Фискализация
	OrderSum        float64   `json:"orderSum"`        // • Сумма заказа до применения скидок
	PaySum          float64   `json:"paySum"`          // • Фактическая сумма заказа, оплаченная пользователем (после применения скидок)
	ItemCount       int       `json:"itemCount"`       // • Кол-во позиций в чеке
	PayType         string    `json:"payType"`         // • Способ оплаты (нал/безнал/иное)
	DiscountSum     float64   `json:"discountSum"`     // • Сумма использованных бонусов/скидок и комментарий по ним
	DiscountComment string    `json:"discountComment"` // • Признак удаления заказа или его сторнирования
}

// Verify checks incoming query authorization.
func (a *App) Verify(r *http.Request) (int, error) {
	if r.Method != http.MethodGet {
		return http.StatusMethodNotAllowed, fmt.Errorf("unsupported request method, IP %s", r.RemoteAddr)
	}

	//check token
	auth := r.Header.Get("Authorization")
	if len(auth) < len(AUTH_PREF) || strings.ToLower(auth[0:len(AUTH_PREF)]) != AUTH_PREF {
		return http.StatusUnauthorized, fmt.Errorf("unauthorized access IP %s", r.RemoteAddr)
	}
	if a.webServerCred != auth[len(AUTH_PREF):] {
		return http.StatusUnauthorized, fmt.Errorf("unauthorized access IP %s", r.RemoteAddr)
	}

	return http.StatusOK, nil
}

// NewQuery is an entry point for incoming queries. Expected parameters:
// date_from, date_to, from, count. From and count params have default values (constants).
// Date_from && date_to do not. If not passed in query, error will be issued.
func (a *App) NewQuery(w http.ResponseWriter, r *http.Request) {
	if err_stat, err := a.Verify(r); err != nil {
		a.Log.Errorf("a.Verify() failed: %v", err)
		w.WriteHeader(err_stat)
		return
	}

	a.Log.Debugf("new query from: %s", r.RemoteAddr)

	var (
		err             error
		param_from      int
		param_count     int
		param_date_from time.Time
		param_date_to   time.Time
	)
	params := r.URL.Query()

	from_s := params.Get("from")
	if from_s != "" {
		param_from, err = strconv.Atoi(from_s)
		if err != nil {
			a.Log.Errorf("Atoi failed:%v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		param_from = DEF_PARAM_FROM
	}
	count_s := params.Get("count")
	if count_s != "" {
		param_count, err = strconv.Atoi(count_s)
		if err != nil {
			a.Log.Errorf("Atoi failed:%v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	} else {
		param_count = DEF_PARAM_COUNT
	}

	date_from_s := params.Get("date_from")
	if date_from_s == "" {
		a.Log.Error("Date_from from parameter not set")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	param_date_from, err = time.Parse(PARAM_DATE_LAYOUT, date_from_s)
	if err != nil {
		a.Log.Errorf("time.Parse() failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	date_to_s := params.Get("date_to")
	if date_to_s == "" {
		a.Log.Error("Date_to to parameter not set")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	param_date_to, err = time.Parse(PARAM_DATE_LAYOUT, date_to_s)
	if err != nil {
		a.Log.Errorf("time.Parse() failed: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rk_date, err := a.FetchRKData(r.Context(), a.Config.MSCon, param_from, param_count, param_date_from, param_date_to)
	if err != nil {
		a.Log.Errorf("FetchRKData() failed:%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	a.MakeResponse(w, rk_date)
}

// MakeResponse constructs http response from data structure and adds to writer.
func (a *App) MakeResponse(w http.ResponseWriter, rkData []RKDate) {
	resp, err := json.Marshal(rkData)
	if err != nil {
		a.Log.Errorf("json.Marshal() failed:%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//writing response
	if _, err := w.Write(resp); err != nil {
		a.Log.Errorf("w.Write() failed:%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// FetchRKData builds ms query and executes it.
func (a *App) FetchRKData(ctx context.Context, msConStr string, from, count int, dateFrom, dateTo time.Time) ([]RKDate, error) {
	var rk_data []RKDate

	db, err := sql.Open("sqlserver", msConStr)
	if err != nil {
		return rk_data, fmt.Errorf("sql.Open() failed: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// if err := db.PingContext(ctx); err != nil {
	// return rk_data, fmt.Errorf("db.PingContext() failed: %v", err)
	// }

	q, err := a.QueryText(from, count, dateFrom, dateTo)
	if err != nil {
		return rk_data, fmt.Errorf("a.QueryText() failed: %v", err)
	}

	a.Log.Debugf("FetchRKData(), query: %s\n", q)

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return rk_data, fmt.Errorf("db.Query() failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		row_data := RKDate{}
		if err := rows.Scan(&row_data.CheckOpen,
			&row_data.CheckClose,
			&row_data.VisitStartTime,
			&row_data.OrderNum,
			&row_data.FiscDocNum,
			&row_data.OrderSum,
			&row_data.PaySum,
			&row_data.ItemCount,
			&row_data.PayType,
			&row_data.DiscountSum,
			&row_data.DiscountComment,
		); err != nil {
			return rk_data, fmt.Errorf("rows.Scan() failed: %v", err)
		}
		rk_data = append(rk_data, row_data)
	}

	return rk_data, nil
}

// QueryText takes query text from file, adds conditions from Config,
// from, to from http query
func (a *App) QueryText(from, count int, dateFrom, dateTo time.Time) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("os.Getwd() faile: %v", err)
	}
	query_b, err := os.ReadFile(filepath.Join(dir, SQL_FILE_NAME))
	if err != nil {
		return "", fmt.Errorf("os.ReadFile() failed: %v", err)
	}

	//
	query_t := string(query_b)
	query_t = strings.Replace(query_t, "{{COUNT}}", fmt.Sprintf("%d", count), 1)
	query_t = strings.Replace(query_t, "{{FROM}}", fmt.Sprintf("%d", from), 1)
	query_t = strings.Replace(query_t, "{{DATE_FROM}}", fmt.Sprintf("'%s'", dateFrom.Format(PARAM_DATE_LAYOUT)), 1)
	query_t = strings.Replace(query_t, "{{DATE_TO}}", fmt.Sprintf("'%s'", dateTo.Format(PARAM_DATE_LAYOUT)), 1)

	//extra conditions
	cond := ""
	if a.sqlFilter != "" {
		cond = " AND " + a.sqlFilter
	}
	query_t = strings.Replace(query_t, "{{FILTER}}", cond, 1)

	return query_t, nil
}
