package main

import (
	// "encoding/json"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

var target string

type Registry map[int]interface{}

var endpointRegistry Registry

func PopulateModels(data map[string]interface{}, conn *pgx.Conn, app_id int) error {
	for key, model := range data {
		f_data, err := json.Marshal(model)
		mdl := &Model{
			name:   key,
			fields: string(f_data),
		}

		args := pgx.NamedArgs{
			"name":   mdl.name,
			"fields": mdl.fields,
			"app_id": app_id,
		}

		_, err = conn.Exec(context.Background(), "insert into Model(name, fields, app_id) values(@name, @fields, @app_id)", args)

		if err != nil {
			log.Println("Failing", err)
		}
	}

	return nil
}

func PopulateAPIAndResponse(data map[string]interface{}, conn *pgx.Conn, app_id int) error {
	for path, api_methods := range data {

		api_methods, isOk := api_methods.(map[string]interface{})

		if !isOk {
			return errors.New("Unable to parse API details")
		}

		// Add to inventory
		args := pgx.NamedArgs{
			"name":   path,
			"app_id": app_id,
		}
		_, err := conn.Exec(context.Background(), "insert into Inventory(name, path, app_id) values(@name, @name, @app_id)", args)

		if err != nil {
			log.Println("inserting in inven", err)
		}

		var inv_id int
		err = conn.QueryRow(context.Background(), "select id from Inventory where name=$1 AND app_id=$2", path, app_id).Scan(&inv_id)
		if err != nil {
			log.Println("selecting from inven", err)
		}

		// Add all api methods

		for req_type, dt := range api_methods {

			dt, isOk := dt.(map[string]interface{})

			if !isOk {
				return errors.New("Unable to parse data")
			}

			parm_string, err := json.Marshal(dt["parameters"])

			if err != nil {
				return err
			}

			responses, err := json.Marshal(dt["responses"])

			tm := time.Now().Unix()
			log.Println(tm)

			args := pgx.NamedArgs{
				"name":      path,
				"path":      path,
				"req_type":  req_type,
				"desc":      dt["description"],
				"time":      tm,
				"params":    parm_string,
				"id":        inv_id,
				"responses": responses,
			}

			// Compare previous API params, response and description

			_, err = conn.Exec(context.Background(),
				"insert into api(name, description, path, parameters, created_time, inventory, req_type, responses) values (@name, @desc, @path, @params, @time, @id, @req_type, @responses)", args)

			if err != nil {
				log.Println("inserting api", err)

				var params string
				var response string
				var id int
				var description string

				err = conn.QueryRow(context.Background(),
					"select id, responses, parameters, description from api where name=$1 and inventory=$2", path, inv_id).Scan(&id, &response, &params, &description)

				if err != nil {
					return err
				}

				if params != string(parm_string) && description != dt["description"] && response != string(responses) {

					tm := time.Now().Unix()
					// update API
					args = pgx.NamedArgs{
						"name":      path,
						"path":      path,
						"req_type":  req_type,
						"desc":      dt["description"],
						"time":      tm,
						"params":    parm_string,
						"id":        inv_id,
						"responses": responses,
						"oid":       id,
					}

					_, err = conn.Exec(context.Background(),
						"update table api set description=@desc, time=@tm, responses=@responses, parameters=@params where inventory=@id and id=@oid", args)

					if err != nil {
						log.Println("Update API error ", err)
						return err
					}

					log.Println("Successfully updated!")
				}
			}

		}
		endpoints := endpointRegistry[app_id].(map[string]bool)
		endpoints[path] = true
		log.Println(endpoints)
		endpointRegistry[app_id] = endpoints
	}

	return nil
}

func PopulateRegistry(conn *pgx.Conn, app_id int) error {
	endpoints := make(map[string]bool)
	var inventoryIds []uint64

	rows, err := conn.Query(context.Background(), "select id from inventory where app_id=$1", app_id)
	defer rows.Close()

	for rows.Next() {
		var id uint64
		err = rows.Scan(&id)
		log.Println(id)
		if err != nil {
			log.Println("Failing here -> ", err)
			return err
		}
		inventoryIds = append(inventoryIds, id)
	}

	if err != nil {
		log.Println("Error", err)
		return err
	}

	for _, id := range inventoryIds {
		var data string
		err = conn.QueryRow(context.Background(), "select path from api where inventory=$1", id).Scan(&data)
		log.Println(data)
		if err != nil {
			log.Println("While getting api")
			return err
		}
		endpoints[data] = false
	}

	endpointRegistry[app_id] = endpoints
	log.Println(endpointRegistry)

	return nil
}

func main() {

	endpointRegistry = make(Registry)

	conn, err := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5432/api_inventory")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	port := flag.String("port", "5000", "Port of application")
	flag.Parse()

	resp, err := http.Get(fmt.Sprintf("http://0.0.0.0:%s/swagger.json", *port))
	// create new application

	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("JSON", err)
	}

	var data map[string]interface{}

	json.Unmarshal(body, &data)

	info, isOk := data["info"].(map[string]interface{})
	if !isOk {
		log.Fatal("Unable to get info")
	}

	log.Println(info["title"])

	var app_id int
	err = conn.QueryRow(context.Background(), "select id from Application where name=$1", info["title"]).Scan(&app_id)
	newEndpoint := false

	if app_id == 0 {
		args := &pgx.NamedArgs{
			"name": info["title"],
		}
		_, err = conn.Exec(context.Background(), "insert into application(name) values(@name)", args)
		if err != nil {
			log.Fatal("unable to create application", err)
		}

		conn.QueryRow(context.Background(), "select id from Application where name=$1", info["title"]).Scan(&app_id)
		newEndpoint = true
		log.Println("New endpoint")
	}

	if !newEndpoint {
		log.Println("Populating..")
		PopulateRegistry(conn, app_id)
	}

	err = PopulateModels(data["definitions"].(map[string]interface{}), conn, app_id)
	if err != nil {
		log.Fatal(err)
	}

	err = PopulateAPIAndResponse(data["paths"].(map[string]interface{}), conn, app_id)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(endpointRegistry)
}
