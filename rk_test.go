package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
			RestaurantId:    111,
			CashGroupId:     222,
			VisitId:         333,
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
		if resp_date[i].RestaurantId != rk_data[i].RestaurantId {
			t.Fatalf("Line %d, RestaurantId expected to be %d, got %d", i, rk_data[i].RestaurantId, resp_date[i].RestaurantId)
		}
		if resp_date[i].CashGroupId != rk_data[i].CashGroupId {
			t.Fatalf("Line %d, CashGroupId expected to be %d, got %d", i, rk_data[i].CashGroupId, resp_date[i].CashGroupId)
		}
		if resp_date[i].VisitId != rk_data[i].VisitId {
			t.Fatalf("Line %d, VisitId expected to be %d, got %d", i, rk_data[i].VisitId, resp_date[i].VisitId)
		}
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

func actTimeAsStr(now time.Time, min int) string {
	return now.Add(time.Duration(min) * time.Minute).Format("15:04")
}

func actTime(now time.Time, min int) time.Time {
	return now.Add(time.Duration(min) * time.Minute).Truncate(time.Minute)
}

func TestNextActDate(t *testing.T) {
	app := NewApp()
	now := time.Now().Truncate(time.Minute)
	// now = now.Add(time.Duration(-10) * time.Minute)
	tests := []struct {
		actTime     string
		actDateTime time.Time
		minCount    int
	}{
		{actTimeAsStr(now, 15), actTime(now, 15), 15},
		{actTimeAsStr(now, 1440), actTime(now, 1440), 1440},
	}
	for _, tt := range tests {
		t.Run(tt.actTime, func(t *testing.T) {
			act_dt, err := app.NextActDate(now, tt.actTime)
			if err != nil {
				t.Fatalf("NextActDate() failed: %v", err)
			}
			if act_dt != tt.actDateTime {
				t.Fatalf("Expected act time: %v, got: %v", tt.actDateTime, act_dt)
			}
			t.Logf("next act time in %d minutes: %v - OK", tt.minCount, act_dt)
		})
	}
}

func TestFetchReportPeriod(t *testing.T) {
	host := ":8081"
	cat := "/period"
	url := "http://" + host + cat
	date_from := time.Now().Add(time.Duration(-10) * 24 * time.Hour).Truncate(time.Minute)
	date_to := time.Now().Truncate(time.Minute)

	http.HandleFunc(cat, func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"dateFrom": "%s", "dateTo":"%s"}`,
			date_from.Format(ReportPeriodLoyout),
			date_to.Format(ReportPeriodLoyout),
		)))
	})
	go http.ListenAndServe(host, nil)

	app := NewApp()
	if err := app.initLogger(); err != nil {
		t.Fatalf("initLogger() failed: %v", err)
	}

	per, err := app.FetchReportPerod(url)
	if err != nil {
		t.Fatalf("FetchReportPerod() failed: %v", err)
	}
	if time.Time(per.DateFrom).Format(ReportPeriodLoyout) != date_from.Format(ReportPeriodLoyout) {
		t.Fatalf("DateFrom got %v, expected %v", time.Time(per.DateFrom).Format(ReportPeriodLoyout), date_from.Format(ReportPeriodLoyout))
	}

	if time.Time(per.DateTo).Format(ReportPeriodLoyout) != date_to.Format(ReportPeriodLoyout) {
		t.Fatalf("DateTo got %v, expected %v", time.Time(per.DateTo).Format(ReportPeriodLoyout), date_to.Format(ReportPeriodLoyout))
	}
}

func TestSendData(t *testing.T) {
	host := ":8085"
	cat := "/data"
	url := "http://" + host + cat

	rk_data := []RKDate{
		{RestaurantId: 1, PaySum: 5000},
		{RestaurantId: 2, PaySum: 10000},
	}

	http.HandleFunc(cat, func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("io.ReadAll() failed:%v", err)
		}

		var rk_got []RKDate
		if err := json.Unmarshal(body, &rk_got); err != nil {
			t.Fatalf("json.Unmarshal() failed:%v", err)
		}
		if len(rk_got) != len(rk_data) {
			t.Fatalf("rk data expected to be %d, got %d", len(rk_data), len(rk_got))
		}
		if rk_got[0].RestaurantId != rk_data[0].RestaurantId {
			t.Fatalf("rk RestaurantId expected to be %d, got %d", rk_data[0].RestaurantId, rk_got[0].RestaurantId)
		}
		if rk_got[0].PaySum != rk_data[0].PaySum {
			t.Fatalf("rk PaySum expected to be %f, got %f", rk_data[0].PaySum, rk_got[0].PaySum)
		}
	})

	go http.ListenAndServe(host, nil)

	app := NewApp()
	if err := app.initLogger(); err != nil {
		t.Fatalf("initLogger() failed: %v", err)
	}
	app.SendData(rk_data, url)

}
