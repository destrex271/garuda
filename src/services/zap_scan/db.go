package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func GetNewEndpoints(conn *pgx.Conn) (map[string]API, error) {
	// get all new endpoints
	rows, err := conn.Query(context.Background(), "select id, name, description, path, created_time, req_type, inventory, responses, operationid, produces, is_new from api where is_new=$1", true)

	if err != nil {
		log.Fatal("ERR", err)
		return nil, err
	}

	var apiMap = make(map[string]API)

	for rows.Next() {
		api := new(API)
		err = rows.Scan(&api.id, &api.name, &api.description, &api.path, &api.createdDate, &api.reqType, &api.inventory, &api.responses, &api.operationid, &api.produces, &api.is_new)
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

func PopulateTestResults(api_data map[string]interface{}, endpoints map[string]API, conn *pgx.Conn) {
	if len(api_data) == 0 {
		for _, data := range endpoints {

			cr_time := time.Now().Unix()
			args := &pgx.NamedArgs{
				"inv":     data.inventory,
				"api":     data.id,
				"results": fmt.Sprintf("[{\"status\": \"ok\", \"created_time\": \"%d\"}]", cr_time),
			}
			_, err := conn.Exec(context.Background(), "insert into test_results(inventory, api_id, results) values(@inv, @api, @results)", args)

			if err != nil {
				// Get previous jsonb data and append as
				var res_data string
				err = conn.QueryRow(context.Background(), "select results from test_results where inventory=@inv and api_id=@api", args).Scan(&res_data)
				args := &pgx.NamedArgs{
					"inv":     data.inventory,
					"api":     data.id,
					"results": fmt.Sprintf("[%s,{\"status\": \"ok\", \"created_time\": \"%d\"}]", res_data, cr_time),
				}
				_, err = conn.Exec(context.Background(), "update test_results set results=@results where inventory=@inv and api_id=@api", args)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
		return
	}

	for key, data := range endpoints {
		results := api_data[key+"_"+data.reqType]
		var res string
		if results == nil {
			res = "[{\"status\": \"ok\"}]"
		} else {
			bts, err := json.Marshal(results)
			if err != nil {
				log.Fatal("ER", err)
			}
			res = string(bts)
		}

		args := &pgx.NamedArgs{
			"inv":     data.inventory,
			"api":     data.id,
			"results": res,
		}
		_, err := conn.Exec(context.Background(), "insert into test_results(inventory, api_id, results) values(@inv, @api, @results)", args)
		if err != nil {
			log.Println(err)
			var res_data string
			err = conn.QueryRow(context.Background(), "select results from test_results where inventory=@inv and api_id=@api", args).Scan(&res_data)
			args := &pgx.NamedArgs{
				"inv":     data.inventory,
				"api":     data.id,
				"results": fmt.Sprintf("[%s,%s]", res_data, res),
			}
			_, err = conn.Exec(context.Background(), "update test_results set results=@results where inventory=@inv and api_id=@api", args)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	log.Println("Updated!")
}

func UpdateEndpointAsOld(conn *pgx.Conn, api API) (bool, error) {
	args := &pgx.NamedArgs{
		"id": api.id,
	}
	_, err := conn.Exec(context.Background(), "update api set is_new=false where id = @id", args)
	if err != nil {
		return false, err
	}
	return true, nil
}
