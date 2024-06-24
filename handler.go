package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	RestaurantId    int       `json:"restaurantId"`
	CashGroupId     int       `json:"cashGroupId"`
	VisitId         int       `json:"visitId"`
	CheckOpen       time.Time `json:"checkOpen"`       // • Дата/время открытия/закрытия заказа
	CheckClose      time.Time `json:"checkClose"`      // • Дата/время открытия/закрытия заказа
	VisitStartTime  time.Time `json:"visitStartTime"`  // • Дата/время формирования пречека
	OrderNum        string    `json:"orderNum"`        // • Номер заказа
	FiscDocNum      string    `json:"fiscDocNum"`      // • Фискализация
	OrderSum        float64   `json:"orderSum"`        // • Сумма заказа до применения скидок
	PaySum          float64   `json:"paySum"`          // • Фактическая сумма заказа, оплаченная пользователем (после применения скидок)
	ItemCount       float64   `json:"itemCount"`       // • Кол-во позиций в чеке
	PayType         string    `json:"payType"`         // • Способ оплаты (нал/безнал/иное)
	DiscountSum     float64   `json:"discountSum"`     // • Сумма использованных бонусов/скидок и комментарий по ним
	DiscountComment string    `json:"discountComment"` // • Признак удаления заказа или его сторнирования
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
		if err := rows.Scan(&row_data.RestaurantId,
			&row_data.CashGroupId,
			&row_data.VisitId,
			&row_data.CheckOpen,
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
