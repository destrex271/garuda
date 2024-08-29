package main

import (
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

var endpointRegistry = make(Registry)
var modelsRegistry = make(Registry)

func PopulateModels(data map[string]interface{}, conn *pgx.Conn, appID int) error {
	models, isOk := modelsRegistry[appID].(map[string]bool)
	if !isOk {
		return errors.New("unable to extract existing models")
	}

	for key, model := range data {
		fData, err := json.Marshal(model)
		if err != nil {
			return err
		}
		mdl := &Model{
			name:   key,
			fields: string(fData),
		}

		args := pgx.NamedArgs{
			"name":   mdl.name,
			"fields": mdl.fields,
			"app_id": appID,
		}

		_, err = conn.Exec(context.Background(), "INSERT INTO Model(name, fields, app_id) VALUES(@name, @fields, @app_id)", args)
		if err != nil {
			log.Println("Error inserting model:", err)
			// Let's check if any updates are requried to the fields or not
			// var prev_fields string
			// err = conn.QueryRow(context.Background(), "Select fields from Model where name=$1 AND app_id=$2", mdl.name, appID).Scan(&prev_fields)
			// log.Println(prev_fields, "--", mdl.fields)
			// if prev_fields == mdl.fields && len(prev_fields) == len(mdl.fields) {
			// 	log.Println("No updates required for model ", mdl.name)
			// 	return nil
			// }
			// TODO: Data comming from DB is not formatted as the data fromatted by json.Marshall
			// Let's run an update query
			_, err = conn.Exec(context.Background(), "UPDATE Model set fields=@fields where app_id=@app_id AND name=@name", args)
			if err != nil {
				log.Println("Error updating model table")
				return err
			}
		}

		delete(models, mdl.name)
	}

	modelsRegistry[appID] = models
	return nil
}

func PopulateAPIAndResponse(data map[string]interface{}, conn *pgx.Conn, appID int) error {
	for path, apiMethods := range data {
		apiMethods, isOk := apiMethods.(map[string]interface{})
		if !isOk {
			return errors.New("unable to parse API details")
		}

		// Add to inventory
		args := pgx.NamedArgs{
			"name":   path,
			"app_id": appID,
		}
		_, err := conn.Exec(context.Background(), "INSERT INTO Inventory(name, path, app_id) VALUES(@name, @name, @app_id)", args)
		if err != nil {
			log.Println("Error inserting into inventory:", err)
		}

		var invID int
		err = conn.QueryRow(context.Background(), "SELECT id FROM Inventory WHERE name=$1 AND app_id=$2", path, appID).Scan(&invID)
		if err != nil {
			log.Println("Error selecting from inventory:", err)
		}

		// Add all API methods
		for reqType, dt := range apiMethods {
			dt, isOk := dt.(map[string]interface{})
			if !isOk {
				return errors.New("unable to parse data")
			}

			parmString, err := json.Marshal(dt["parameters"])
			if err != nil {
				return err
			}

			responses, err := json.Marshal(dt["responses"])
			if err != nil {
				return err
			}

			reqBody, err := json.Marshal(dt["requestBody"])

			tm := time.Now().Unix()

			opid := dt["operationid"]
			prod := dt["produces"]

			if prod != nil {
				prod, err = json.Marshal(prod)
				if err != nil {
					log.Println("Unable to marshal ", err)
					return err
				}
			}

			args = pgx.NamedArgs{
				"name":        path,
				"path":        path,
				"req_type":    reqType,
				"desc":        dt["description"],
				"time":        tm,
				"params":      parmString,
				"id":          invID,
				"responses":   responses,
				"operationid": opid,
				"produces":    prod,
				"reqb":        reqBody,
			}

			// Check if API already exists or not
			var id int
			err = conn.QueryRow(context.Background(),
				"SELECT id FROM api WHERE name=$1 AND inventory=$2 AND req_type=$3", path, invID, reqType).Scan(&id)

			if id == 0 {
				_, err = conn.Exec(context.Background(),
					"INSERT INTO api(name, description, path, parameters, created_time, inventory, req_type, responses, operationid, produces, reqb) VALUES (@name, @desc, @path, @params, @time, @id, @req_type, @responses, @operationid, @produces, @reqb)", args)
			} else {
				log.Println("Updating endpoint")
				// Update endpoint
				var params, response, description string
				var id int
				err = conn.QueryRow(context.Background(),
					"SELECT id, responses, parameters, description FROM api WHERE name=$1 AND inventory=$2", path, invID).Scan(&id, &response, &params, &description)
				if err != nil {
					log.Println("selecting previous", err)
				}

				if params != string(parmString) || description != dt["description"] || response != string(responses) {
					tm := time.Now().Unix()
					args = pgx.NamedArgs{
						"name":      path,
						"path":      path,
						"req_type":  reqType,
						"desc":      dt["description"],
						"time":      tm,
						"params":    parmString,
						"id":        invID,
						"responses": responses,
						"oid":       id,
						"reqb":      reqBody,
					}

					_, err = conn.Exec(context.Background(),
						"UPDATE api SET description=@desc, created_time=@time, responses=@responses, parameters=@params, reqb=@reqb WHERE inventory=@id AND id=@oid", args)
					if err != nil {
						log.Println("Error updating API:", err)
						return err
					}

					log.Println("Successfully updated API!")
				}
			}
		}

		if endpoints, ok := endpointRegistry[appID].(map[string]bool); ok {
			delete(endpoints, path)
			endpointRegistry[appID] = endpoints
		}
	}

	return nil
}

func PopulateRegistry(conn *pgx.Conn, appID int) error {
	endpoints := make(map[string]bool)
	var inventoryIDs []int

	rows, err := conn.Query(context.Background(), "SELECT id FROM Inventory WHERE app_id=$1", appID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			log.Println("Error scanning inventory ID:", err)
			return err
		}
		inventoryIDs = append(inventoryIDs, id)
	}

	for _, id := range inventoryIDs {
		var path string
		err = conn.QueryRow(context.Background(), "SELECT path FROM API WHERE inventory=$1", id).Scan(&path)
		if err != nil {
			log.Println("Error getting API path:", err)
			return err
		}
		endpoints[path] = false
	}

	endpointRegistry[appID] = endpoints
	log.Println(endpointRegistry)

	return nil
}

func PopulateModelRegistry(conn *pgx.Conn, appID int) error {
	models := make(map[string]bool)

	rows, err := conn.Query(context.Background(), "SELECT name FROM Model WHERE app_id=$1", appID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			log.Println("Error scanning model name:", err)
			return err
		}
		models[name] = false
	}

	modelsRegistry[appID] = models
	log.Println(modelsRegistry)

	return nil
}

func DeleteNonExistentEndpoints(conn *pgx.Conn, appID int) error {
	endpoints, ok := endpointRegistry[appID].(map[string]bool)
	if !ok {
		log.Println("Error: Unable to extract endpoints from registry")
		return errors.New("unable to extract endpoints")
	}
	log.Println("Endpoints to delete:", endpoints)

	const query = "DELETE FROM Inventory WHERE name=$1"

	for key := range endpoints {
		log.Println("Deleting ->", key)

		_, err := conn.Exec(context.Background(), query, key)
		if err != nil {
			log.Println("Error executing delete:", err)
			return err
		}
	}

	return nil
}

func DeleteNonExistentModels(conn *pgx.Conn, appID int) error {
	models, ok := modelsRegistry[appID].(map[string]bool)
	if !ok {
		log.Println("Error: Unable to extract models from registry")
		return errors.New("unable to extract models")
	}
	log.Println("Models to delete:", models)

	const query = "DELETE FROM Model WHERE name=$1"

	for key := range models {
		log.Println("Deleting ->", key)

		_, err := conn.Exec(context.Background(), query, key)
		if err != nil {
			log.Println("Error executing delete:", err)
			return err
		}
	}
	return nil
}

func main() {
	conn, err := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5432/api_inventory")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	port := flag.String("port", "5000", "Port of application")
	json_file_url := flag.String("apiDocs", fmt.Sprintf("http://0.0.0.0:%s/swagger.json", *port), "URL for OpenAPI specification json.")
	flag.Parse()

	resp, err := http.Get(*json_file_url)
	if err != nil {
		log.Fatal("Error fetching swagger.json:", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response body:", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Fatal("Error unmarshalling JSON:", err)
	}

	info, isOk := data["info"].(map[string]interface{})
	if !isOk {
		log.Fatal("Unable to get info from swagger JSON")
	}

	log.Println("Application title:", info["title"])

	var appID int
	err = conn.QueryRow(context.Background(), "SELECT id FROM Application WHERE name=$1", info["title"]).Scan(&appID)
	if err != nil {
		if err == pgx.ErrNoRows {
			args := pgx.NamedArgs{
				"name": info["title"],
			}
			_, err = conn.Exec(context.Background(), "INSERT INTO Application(name) VALUES(@name)", args)
			if err != nil {
				log.Fatal("Unable to create application:", err)
			}

			err = conn.QueryRow(context.Background(), "SELECT id FROM Application WHERE name=$1", info["title"]).Scan(&appID)
			if err != nil {
				log.Fatal("Error retrieving new application ID:", err)
			}

			log.Println("New application created")
		} else {
			log.Fatal("Error querying application:", err)
		}
	} else {
		log.Println("Application already exists")
	}

	log.Println("Populating registries...")
	err = PopulateRegistry(conn, appID)
	if err != nil {
		log.Fatal("Error populating registry:", err)
	}

	err = PopulateModelRegistry(conn, appID)
	if err != nil {
		log.Fatal("Error populating model registry:", err)
	}

	err = PopulateModels(data["definitions"].(map[string]interface{}), conn, appID)
	if err != nil {
		log.Fatal("Error populating models:", err)
	}

	err = PopulateAPIAndResponse(data["paths"].(map[string]interface{}), conn, appID)
	if err != nil {
		log.Println("Error populating API and response:", err)
	}

	err = DeleteNonExistentEndpoints(conn, appID)
	if err != nil {
		log.Println("Error deleting non-existent endpoints:", err)
	}

	err = DeleteNonExistentModels(conn, appID)
	if err != nil {
		log.Fatal("Error deleting non-existent models:", err)
	}

	log.Println("Final endpoint registry:", endpointRegistry)
	log.Println("Final models registry:", modelsRegistry)
}
