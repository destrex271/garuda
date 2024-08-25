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

func PopulateModels(data map[string]interface{}, conn *pgx.Conn) error{
    for key, model := range data{
        f_data, err := json.Marshal(model)
        mdl := &Model{
            name: key,
            fields: string(f_data),
        }

        args := pgx.NamedArgs{
            "name":    mdl.name,
            "fields":   mdl.fields,
        }

        _, err = conn.Exec(context.Background(), "insert into Model(name, fields) values(@name, @fields)", args)
       
        if err != nil{
            log.Println("Failing")
            return err
        }
    }

    return nil
}

func PopulateAPIAndResponse(data map[string]interface{}, conn *pgx.Conn) error{
    for path, api_methods := range data{

        api_methods, isOk := api_methods.(map[string]interface{})

        if !isOk{
            return errors.New("Unable to parse API details")
        }

        // Add to inventory
        args := pgx.NamedArgs{
            "name": path,
        }
        _, err := conn.Exec(context.Background(), "insert into Inventory(name, path) values(@name, @name)", args)

        if err != nil{
            log.Println("inserting in inven")
            return err
        }
        
        var inv_id int
        err = conn.QueryRow(context.Background(), "select id from Inventory where name=$1", path).Scan(&inv_id)
        if err != nil{
            log.Println("selecting from inven")
            return err
        }

        // Add all api methods

        for req_type, dt := range api_methods{

            dt, isOk := dt.(map[string]interface{})
            
            if !isOk{
                return errors.New("Unable to parse data")
            }

            parm_string, err := json.Marshal(dt["parameters"])

            if err != nil{
                return err
            }

            responses, err := json.Marshal(dt["responses"])

            tm := time.Now().Unix()
            log.Println(tm)

            args := pgx.NamedArgs{
                "name": path,
                "path": path,
                "req_type": req_type,
                "desc": dt["description"],
                "time": tm,
                "params": parm_string,
                "id": inv_id,
                "responses": responses,
            }

            _, err = conn.Exec(context.Background(), 
            "insert into api(name, description, path, parameters, created_time, inventory, req_type, responses) values (@name, @desc, @path, @params, @time, @id, @req_type, @responses)", args)

            if err != nil{
                log.Println("inserting api")
                return err
            }
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
	flag.Parse()

    resp, err := http.Get(fmt.Sprintf("http://0.0.0.0:%s/swagger.json", *port))

    if err != nil{
        log.Fatal(err)
    }

    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil{
        log.Fatal("JSON", err)
    }

    var data map[string]interface{}

    json.Unmarshal(body, &data)

    err = PopulateModels(data["definitions"].(map[string]interface{}), conn)
    if err != nil{
        log.Fatal(err)
    }

    err = PopulateAPIAndResponse(data["paths"].(map[string]interface{}), conn)
    if err != nil{
        log.Fatal(err)
    }
}
