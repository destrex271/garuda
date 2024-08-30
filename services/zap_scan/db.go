package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Alert struct {
	Name        string `json:"alert"`
	Confidence  string `json:"confidence"`
	CWEID       string `json:"cweid"`
	Description string `json:"desc"`
	OtherInfo   string `json:"otherinfo"`
	Reference   string `json:"reference"`
	Solution    string `json:"solution"`
	RiskDesc    string `json:"riskdesc"`
	Status      string `json:"status"`
	CreatedTime string `json:"created_time"`
}

func GetNewEndpointsForApplication(conn *pgx.Conn, app_name string) (map[string]API, error) {
	// get application id
	var app_id int
	err := conn.QueryRow(context.Background(), "select id from application where name=$1", app_name).Scan(&app_id)

	if err != nil {
		log.Fatal(err)
	}

	log.Println(app_id)

	// get inventories related to application id
	arows, err := conn.Query(context.Background(), "select id from inventory where app_id=$1", app_id)
	apiMap := make(map[string]API)
	var inv_ids []int

	for arows.Next() {
		var inv_id int
		arows.Scan(&inv_id)

		// get all new endpoints
		inv_ids = append(inv_ids, inv_id)
	}

	for _, inv_id := range inv_ids {
		log.Println("Querying inventory -> ", inv_id)
		rows, err := conn.Query(context.Background(), "select id, name, description, path, created_time, req_type, inventory, responses, operationid, produces, is_new, reqb from api where is_new=$1 and inventory=$2", true, inv_id)

		if err != nil {
			log.Fatal("ERR", err)
			return nil, err
		}

		for rows.Next() {
			api := new(API)
			err = rows.Scan(&api.id, &api.name, &api.description, &api.path, &api.createdDate, &api.reqType, &api.inventory, &api.responses, &api.operationid, &api.produces, &api.is_new, &api.reqb)
			log.Println(api)
			if err != nil {
				log.Println("ERROR", err)
				continue // Skip to next row on error
			}

			// Use path + req_type as the map key
			key := api.path + "_" + api.reqType
			apiMap[key] = *api
		}
	}

	return apiMap, nil
}

func GetNewEndpoints(conn *pgx.Conn) (map[string]API, error) {
	// get all new endpoints
	rows, err := conn.Query(context.Background(), "select id, name, description, path, created_time, req_type, inventory, responses, operationid, produces, is_new, reqb from api where is_new=$1", true)

	if err != nil {
		log.Fatal("ERR", err)
		return nil, err
	}

	var apiMap = make(map[string]API)

	for rows.Next() {
		api := new(API)
		err = rows.Scan(&api.id, &api.name, &api.description, &api.path, &api.createdDate, &api.reqType, &api.inventory, &api.responses, &api.operationid, &api.produces, &api.is_new, &api.reqb)
		if err != nil {
			log.Println("ERROR", err)
			continue // Skip to next row on error
		}

		// Use path + req_type as the map key
		key := api.path + "_" + api.reqType
		apiMap[key] = *api
	}

	return apiMap, nil
}

func PopulateTestResultsSingleScan(alerts_map map[string]interface{}, endpoints map[string]API, conn *pgx.Conn) error {

	checklist := make(map[string]bool)

	for key, _ := range endpoints {
		checklist[key] = false
	}

	created_time := time.Now().Unix()
	site_data := alerts_map["site"].([]interface{})
	re := regexp.MustCompile(`^https?://`)

	var alerts []interface{}

	for _, site := range site_data {
		data, _ := site.(map[string]interface{})
		name, _ := data["@name"].(string)
		if name != baseURL {
			return nil
		}

		alerts, _ = data["alerts"].([]interface{})
	}

	// log.Fatal(alerts)

	for _, alert := range alerts {
		alert_dt, _ := alert.(map[string]interface{})
		riskcode, _ := alert_dt["riskcode"].(string)
		if riskcode == "0" {
			continue
		}
		alert_obj := Alert{
			Name:        alert_dt["alert"].(string),
			Confidence:  alert_dt["confidence"].(string),
			CWEID:       alert_dt["cweid"].(string),
			Description: alert_dt["desc"].(string),
			OtherInfo:   alert_dt["otherinfo"].(string),
			Reference:   alert_dt["reference"].(string),
			Solution:    alert_dt["solution"].(string),
			RiskDesc:    alert_dt["riskdesc"].(string),
			Status:      "failed",
			CreatedTime: strconv.FormatInt(created_time, 10),
		}

		instances := alert_dt["instances"].([]interface{})
		alert_json, err := json.Marshal(alert_obj)

		if err != nil {
			log.Fatal("Unable to convert to json: ", err)
		}

		for _, instance := range instances {
			inst := instance.(map[string]interface{})
			uri := inst["uri"].(string)
			uri = re.ReplaceAllString(uri, "")
			method := strings.ToLower(inst["method"].(string))
			key := uri + "_" + method

			api := endpoints[key]
			log.Println(key, api)
			if api.id == 0 {
				continue
			}
			args := &pgx.NamedArgs{
				"inv":     api.inventory,
				"api":     api.id,
				"results": fmt.Sprintf("[%s]", string(alert_json)),
			}
			log.Println(api.inventory, " ", api.id)
			_, err := conn.Exec(context.Background(), "insert into test_results(inventory, api_id, results) values(@inv, @api, @results)", args)
			if err != nil {
				log.Println(err)
				// Get previous jsonb data and append as
				var res_data string
				err = conn.QueryRow(context.Background(), "select results from test_results where inventory=@inv and api_id=@api", args).Scan(&res_data)
				args := &pgx.NamedArgs{
					"inv":     api.inventory,
					"api":     api.id,
					"results": fmt.Sprintf("[%s,%s]", alert_json, res_data[1:len(res_data)-1]),
				}
				_, err = conn.Exec(context.Background(), "update test_results set results=@results where inventory=@inv and api_id=@api", args)
				if err != nil {
					log.Fatal("FATAL", err)
				} else {
					checklist[key] = true
				}
			} else {
				checklist[key] = true
			}
		}
	}

	// Add Success Result for other endpoints that are not checked
	for key, exists := range checklist {
		log.Println(exists)
		if exists {
			continue
		}
		log.Println("Adding for -> ", key)
		api := endpoints[key]
		args := &pgx.NamedArgs{
			"inv":     api.inventory,
			"api":     api.id,
			"results": fmt.Sprintf("[{\"status\": \"ok\", \"created_time\": \"%d\"}]", created_time),
		}
		_, err := conn.Exec(context.Background(), "insert into test_results(inventory, api_id, results) values(@inv, @api, @results)", args)
		if err != nil {
			log.Println(err)
			var res_data string
			err = conn.QueryRow(context.Background(), "select results from test_results where inventory=@inv and api_id=@api", args).Scan(&res_data)
			args := &pgx.NamedArgs{
				"inv":     api.inventory,
				"api":     api.id,
				"results": fmt.Sprintf("[%s,{\"status\": \"ok\", \"created_time\": \"%d\"}]", res_data[1:len(res_data)-1], created_time),
			}
			_, err = conn.Exec(context.Background(), "update test_results set results=@results where inventory=@inv and api_id=@api", args)
			if err != nil {
				log.Fatal(err)
			} else {
				checklist[key] = true
			}
		} else {
			checklist[key] = true
		}
	}

	log.Println("FIN POPS")
	return nil

}

func UpdateEndpointAsOld(conn *pgx.Conn, id int) (bool, error) {
	args := &pgx.NamedArgs{
		"id": id,
	}
	op, err := conn.Exec(context.Background(), "update api set is_new=false where id = @id", args)
	log.Println(op)
	if err != nil {
		log.Println(err)
		return false, err
	}
	return true, nil
}
