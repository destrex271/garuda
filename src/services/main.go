package main

import (
	// "encoding/json"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

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

        a, err := conn.Exec(context.Background(), "insert into Model(name, fields) values(@name, @fields)", args)
        conn.Exec(context.Background(), "commit;", args)
        log.Println("OK", a)
       
        if err != nil{
            log.Println("Failing")
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
}
