package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/zaproxy/zap-api-go/zap"
)

/**
*
* This script takes the source apiDocs URL and fetches all the endpoints stored in the api_inventory database.
* These urls are passed as the intial urls to the ZAP spider which launches an Active Scan on all the URLs and
* any other URLs found in these pages
*
**/

var target string
var apidocs string
var baseURL string
var appName string
var client zap.Interface
var cfg *zap.Config

func init() {
	// flag.StringVar(&target, "target", "http://localhost:16000/apidocs", "target address")
	flag.StringVar(&apidocs, "apidocs", "/home/akshat/fg/dummyapi2/", "target address")
	flag.StringVar(&baseURL, "baseurl", "http://localhost:16000", "targ")
	flag.StringVar(&appName, "appname", "REST API", "targ")
	cfg = &zap.Config{
		Proxy: "http://127.0.0.1:8081",
	}
	client, _ = zap.NewClient(cfg)

	flag.Parse()
}

func removeHostAndPort(url string) string {
	re := regexp.MustCompile(`^https?://[^/]+`)
	return re.ReplaceAllString(url, "")
}

func GetEndpointsFromAlerts(data map[string]interface{}, endpoints map[string]API) (map[string]interface{}, error) {
	alerts_data := make(map[string]interface{})
	sites, _ := data["site"].([]interface{})

	for _, site := range sites {
		site_map, _ := site.(map[string]interface{})
		alerts, _ := site_map["alerts"].([]interface{})

		for _, alert := range alerts {
			alert_map, _ := alert.(map[string]interface{})
			alert_name := alert_map["alert"].(string)
			confidence := alert_map["confidence"].(string)
			desc := alert_map["desc"].(string)
			instances_data, _ := alert_map["instances"].([]interface{})
			for _, instance := range instances_data {
				inst_data, _ := instance.(map[string]any)
				uri, _ := inst_data["uri"].(string)
				method, _ := inst_data["method"].(string)
				key := removeHostAndPort(uri + "_" + method)
				api := endpoints[key]
				log.Println(key+" -> ", api)
				if api.id > 0 {
					api_data := make(map[string]interface{})
					api_data["alert_name"] = alert_name
					api_data["confidence"] = confidence
					api_data["desc"] = desc
					api_data["status"] = "failing"
					alerts_data[api.name] = api_data
				}
			}
		}
	}

	return alerts_data, nil
}

func ActiveZapScan() (map[string]interface{}, error) {

	spider := client.Spider()
	resp, err := spider.Scan(target, "", "", "", "")

	log.Println("OK", resp)
	scanId := resp["scan"].(string)

	var stat int
	stat = 0
	for stat < 100 {
		resp, err = spider.Status(scanId)
		if err != nil {
			log.Fatal(err)
		}

		stat, _ = strconv.Atoi(resp["status"].(string))
		log.Println(stat)
	}
	log.Println("Scan complete")
	res, err := spider.Results(scanId)
	log.Println(res)

	scanner := client.Ascan()

	resp, err = scanner.Scan(target, "", "", "", "", "", "")

	if err != nil {
		log.Fatal(err)
	}

	log.Println(resp)

	scanID := resp["scan"].(string)

	stat = 0
	for stat < 100 {
		resp, err = scanner.Status(scanID)
		if err != nil {
			log.Fatal(err)
		}

		stat, _ = strconv.Atoi(resp["status"].(string))
		log.Println(stat)
	}
	log.Println("Scan complete")

	report, err := client.Core().Jsonreport()
	if err != nil {
		log.Fatal(err)
	}

	var jsonReport map[string]interface{}
	json.Unmarshal([]byte(report), &jsonReport)

	return jsonReport, nil
}

func LoadSiteMap() {
	apis := client.Openapi()
	apis.ImportFile(apidocs, baseURL, "1")
	log.Println("Loaded sitemap")
}

func ActiveZapScanSingle(url string, nm string) (map[string]interface{}, error) {

	log.Println("Scanning -> ", url)

	spider := client.Spider()
	resp, err := spider.Scan(url, "", "", "", "")

	log.Println("OK", resp)
	scanId := resp["scan"].(string)

	var stat int
	stat = 0
	for stat < 100 {
		resp, err = spider.Status(scanId)
		if err != nil {
			log.Fatal(err)
		}

		stat, _ = strconv.Atoi(resp["status"].(string))
		log.Println(stat)
	}
	log.Println("Scan complete")
	res, err := spider.Results(scanId)
	log.Println(res)

	LoadSiteMap()

	scanner := client.Ascan()

	resp, err = scanner.Scan(url, "", "", "", "", "", "")

	if err != nil {
		log.Fatal(err)
	}

	log.Println(resp)

	scanID := resp["scan"].(string)

	stat = 0
	for stat < 100 {
		resp, err = scanner.Status(scanID)
		if err != nil {
			log.Fatal(err)
		}

		stat, _ = strconv.Atoi(resp["status"].(string))
		log.Println(stat)
	}
	log.Println("Scan complete")

	report, err := client.Core().Jsonreport()
	if err != nil {
		log.Fatal(err)
	}

	var jsonReport map[string]interface{}
	json.Unmarshal([]byte(report), &jsonReport)

	file, err := os.Create(nm + "_result.json")
	if err != nil {
		log.Println("Unable to create file", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "	")
	if err := encoder.Encode(jsonReport); err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	return jsonReport, nil
}

// func main() {

// 	log.Println(target)
// 	LoadSiteMap()
// 	data, err := ActiveZapScan()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	file, err := os.Create("file.json")
// 	encoder := json.NewEncoder(file)
// 	encoder.SetIndent("", "	")
// 	if err := encoder.Encode(data); err != nil {
// 		log.Fatal(err)
// 	}

// 	conn, err := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5432/api_inventory")
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
// 		os.Exit(1)
// 	}
// 	defer conn.Close(context.Background())

// 	endpoints, err := GetNewEndpoints(conn)

// 	alert_data, err := GetEndpointsFromAlerts(data, endpoints)

// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	log.Println(alert_data)

// 	PopulateTestResults(alert_data, endpoints, conn)
// }

func main() {
	log.Println(target)

	// Get all endpoints of application
	conn, err := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5432/api_inventory")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	endpoints, _ := GetNewEndpointsForApplication(conn, appName)
	log.Println(endpoints)

	var alerts map[string]interface{}
	// log.Fatal(endpoints)

	for _, api := range endpoints {
		log.Println("Active scan for ", api.path)
		re := regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`)
		// url = re.ReplaceAllString(url, replacementValue)
		log.Println("Checking ", api.path)
		if re.MatchString(api.path) {
			log.Println("Skipping")
			continue
		}

		if api.path[0:5] != "http" {
			api.path = "http://" + api.path
		}
		LoadSiteMap()

		alerts, err = ActiveZapScanSingle(api.path, strings.ReplaceAll(api.name, "/", "_"))
		if err != nil {
			log.Fatal(err)
		}
		log.Println("OKK")
	}
	PopulateTestResultsSingleScan(alerts, endpoints, conn)

	if len(alerts) == 0 {
		log.Println(alerts)
	}
}
