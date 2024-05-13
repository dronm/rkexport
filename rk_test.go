package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func SetCred(t *testing.T, req *http.Request, cred string) {
	cred_enc := b64.StdEncoding.EncodeToString([]byte(cred))
	req.Header.Set("Authorization", "Basic "+cred_enc)
}

func TestConfig(t *testing.T) {
	config := []byte(`{
		"msCon": "MSCON",
		"restaurants":["Rest1"],
		"cashGroups":["Group1"],
		"logLevel":"debug",
		"webServer":{
			"credential":"andrey:123456",
			"host":":59000",
			"idleTimeout":10000,
			"readTimeout":20000,
			"writeTimeout":30000,
			"handlerTimeout":5000
			}
		}`)
	app := NewApp()
	if err := app.LoadConfig(config); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		Expected interface{}
		Got      interface{}
		Descr    string
	}{
		{Expected: "debug", Got: app.Config.LogLevel, Descr: "logLevel"},
		{Expected: "MSCON", Got: app.Config.MSCon, Descr: "MS connection"},
		{Expected: "andrey:123456", Got: app.Config.WebServer.Credential, Descr: "credential"},
		{Expected: ":59000", Got: app.Config.WebServer.Host, Descr: "webServer host"},
		{Expected: 10000, Got: app.Config.WebServer.IdleTimeout, Descr: "webServer idleTimeout"},
		{Expected: 20000, Got: app.Config.WebServer.ReadTimeout, Descr: "webServer readTimeout"},
		{Expected: 30000, Got: app.Config.WebServer.WriteTimeout, Descr: "webServer writeTimeout"},
		{Expected: 5000, Got: app.Config.WebServer.HandlerTimeout, Descr: "webServer handlerTimeout"},
	}
	for _, ts := range tests {
		if ts.Expected != ts.Got {
			t.Fatalf("%s expected: %v, got: %v", ts.Descr, ts.Expected, ts.Got)
		}
	}
	if len(app.Config.Restaurants) == 0 {
		t.Fatal("Restaurants length expected to be 1")
	}
	if len(app.Config.CashGroups) == 0 {
		t.Fatal("CashGroups length expected to be 1")
	}
	if app.Config.Restaurants[0] != "Rest1" {
		t.Fatalf("%s expected: %v, got: %v", "Restaurant", "Rest1", app.Config.Restaurants[0])
	}
	if app.Config.CashGroups[0] != "Group1" {
		t.Fatalf("%s expected: %v, got: %v", "CashGroup", "Group1", app.Config.CashGroups[0])
	}
}

func TestMakeResponse(t *testing.T) {
	rk_data := []RKDate{
		{CheckClose: time.Now(),
			CheckOpen:       time.Now().Add(time.Duration(1) * time.Minute),
			VisitStartTime:  time.Now(),
			FiscDocNum:      "Fiscalization",
			OrderNum:        "123",
			OrderSum:        123.45,
			PaySum:          50.15,
			ItemCount:       3,
			DiscountSum:     73.3,
			DiscountComment: "discount",
		},
	}
	app := NewApp()
	w := httptest.NewRecorder()
	app.MakeResponse(w, rk_data)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status code: %d, got: %d", http.StatusOK, w.Code)
	}
	// fmt.Println(string(w.Body.String()))
	var resp_date []RKDate
	if err := json.Unmarshal(w.Body.Bytes(), &resp_date); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(resp_date) != len(rk_data) {
		t.Fatalf("response count expected to be %d, got %d", len(rk_data), len(resp_date))
	}
	date_l := "02.01.2006"
	for i := range resp_date {
		if resp_date[i].CheckOpen.Compare(rk_data[i].CheckOpen) != 0 {
			t.Fatalf("Line %d, CheckOpen expected to be %s, got %s", i, rk_data[i].CheckOpen.Format(date_l), resp_date[i].CheckOpen.Format(date_l))

		}
		if resp_date[i].CheckClose.Compare(rk_data[i].CheckClose) != 0 {
			t.Fatalf("Line %d, CheckClose expected to be %s, got %s", i, rk_data[i].CheckClose.Format(date_l), resp_date[i].CheckClose.Format(date_l))

		}
		if resp_date[i].VisitStartTime.Compare(rk_data[i].VisitStartTime) != 0 {
			t.Fatalf("Line %d, VisitStartTime expected to be %s, got %s", i, rk_data[i].VisitStartTime.Format(date_l), resp_date[i].VisitStartTime.Format(date_l))
		}
		if resp_date[i].OrderSum != rk_data[i].OrderSum {
			t.Fatalf("Line %d, OrderSum expected to be %f, got %f", i, rk_data[i].OrderSum, resp_date[i].OrderSum)
		}
		if resp_date[i].OrderNum != rk_data[i].OrderNum {
			t.Fatalf("Line %d, OrderNum expected to be %s, got %s", i, rk_data[i].OrderNum, resp_date[i].OrderNum)
		}
		if resp_date[i].FiscDocNum != rk_data[i].FiscDocNum {
			t.Fatalf("Line %d, FiscDocNum expected to be %s, got %s", i, rk_data[i].FiscDocNum, resp_date[i].FiscDocNum)
		}
		if resp_date[i].DiscountComment != rk_data[i].DiscountComment {
			t.Fatalf("Line %d, DiscountComment expected to be %s, got %s", i, rk_data[i].DiscountComment, resp_date[i].DiscountComment)
		}
		if resp_date[i].DiscountSum != rk_data[i].DiscountSum {
			t.Fatalf("Line %d, DiscountSum expected to be %f, got %f", i, rk_data[i].DiscountSum, resp_date[i].DiscountSum)
		}
	}
}

func TestAuth(t *testing.T) {
	config := []byte(`{
		"webServer":{
			"credential":"andrey:123456"
			}
		}`)
	app := NewApp()
	if err := app.LoadConfig(config); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	tests := []struct {
		Credential         string
		ExpectedStatusCode int
	}{
		{Credential: "andrey:123456", ExpectedStatusCode: http.StatusOK},
		{Credential: "andrey:1234567", ExpectedStatusCode: http.StatusUnauthorized},
	}

	date_to := time.Now().Add(time.Duration(24) * time.Hour)
	date_from := time.Now()
	q_params := "?date_from=" + date_from.Format(PARAM_DATE_LAYOUT) + "&date_to=" + date_to.Format(PARAM_DATE_LAYOUT)

	for _, ts := range tests {
		req := httptest.NewRequest(http.MethodGet, "/"+q_params, nil)
		SetCred(t, req, ts.Credential)
		// w := httptest.NewRecorder()
		if resp_code, err := app.Verify(req); resp_code != ts.ExpectedStatusCode {
			t.Fatalf("Expected status code: %d, got: %d, error:%v", ts.ExpectedStatusCode, resp_code, err)
		}
	}

}

func TestQueryText(t *testing.T) {
	app := NewApp()
	config := []byte(`{
			"restaurants":["Премьер", "Гудвин"],
			"cashGroups":["cashGr1","cashGr2"]
		}`)
	if err := app.LoadConfig(config); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	date_to := time.Now()
	date_from := time.Now().Add(time.Duration(24) * time.Hour)
	q, err := app.QueryText(5, 100, date_from, date_to)
	if err != nil {
		t.Fatalf("app.QueryText() failed: %v", err)
	}
	fmt.Println(q)
	if strings.Index(q, "NEXT 100") == -1 {
		t.Fatal("Expected NEXT 100, found nothing")
	}
	if strings.Index(q, "OFFSET 5") == -1 {
		t.Fatal("Expected OFFSET 5, found nothing")
	}
	if strings.Index(q, " = 'Премьер'") == -1 {
		t.Fatal("Expected  = 'Премьер', found nothing")
	}
	if strings.Index(q, " = 'Гудвин'") == -1 {
		t.Fatal("Expected  = 'Гудвин', found nothing")
	}
	if strings.Index(q, " = 'cashGr1'") == -1 {
		t.Fatal("Expected  = 'cashGr1', found nothing")
	}
	if strings.Index(q, " = 'cashGr2'") == -1 {
		t.Fatal("Expected  = 'cashGr2', found nothing")
	}
}
