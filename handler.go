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

// MSSQL query row result
type RKDate map[string]interface{}

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

	columns, err := rows.Columns()
	if err != nil {
		return rk_data, err
	}

	column_types, err := rows.ColumnTypes()
	if err != nil {
		return rk_data, err
	}

	rk_data = make([]RKDate, 0)

	for rows.Next() {
		values := make([]interface{}, len(columns))
		value_ptrs := make([]interface{}, len(columns))
		for i := range values {
			value_ptrs[i] = &values[i]
		}

		if err := rows.Scan(value_ptrs...); err != nil {
			return rk_data, err
		}

		row_map := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Handle null values
			if val == nil {
				row_map[col] = nil
				continue
			}
			// a.Log.Debugf("column %d, SQL type: %s", i, column_types[i].DatabaseTypeName())
			// Handle different types explicitly
			var converted bool
			col_type := column_types[i].DatabaseTypeName()
			switch col_type {
			case "INT", "BIGINT", "SMALLINT", "TINYINT":
				if row_map[col], converted = val.(int64); !converted {
					return rk_data, fmt.Errorf("error converting %s to int64", col_type)
				}
			case "FLOAT", "REAL", "DECIMAL", "NUMERIC":
				if row_map[col], converted = val.(float64); !converted {
					return rk_data, fmt.Errorf("error converting %s to float64", col_type)
				}
			case "MONEY", "SMALLMONEY":
				var val_b []byte
				if val_b, converted = val.([]byte); !converted {
					return rk_data, fmt.Errorf("error converting %s to byte", col_type)
				}
				val_f, err := strconv.ParseFloat(string(val_b), 64)
				if err != nil {
					return rk_data, fmt.Errorf("error converting %s: strconv.ParseFloat() failed: %v", col_type, err)
				}
				row_map[col] = val_f
			case "BIT":
				if row_map[col], converted = val.(bool); !converted {
					return rk_data, fmt.Errorf("error converting %s to bool", col_type)
				}
			case "CHAR", "VARCHAR", "TEXT", "NCHAR", "NVARCHAR", "NTEXT":
				if row_map[col], converted = val.(string); !converted {
					return rk_data, fmt.Errorf("error converting %s to string", col_type)
				}
			case "DATE", "DATETIME", "DATETIME2", "SMALLDATETIME", "TIME", "DATETIMEOFFSET":
				var val_time time.Time
				if val_time, converted = val.(time.Time); !converted {
					return rk_data, fmt.Errorf("error converting %s to time.Time", col_type)
				} else {
					row_map[col] = val_time.Format(time.RFC3339)
				}
			default:
				a.Log.Debugf("column %d, unknown SQL type: %s", i, col_type)
				row_map[col] = fmt.Sprintf("%v", val)
			}
			// a.Log.Debugf("map value: %v", row_map[col])
		}
		rk_data = append(rk_data, row_map)
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
