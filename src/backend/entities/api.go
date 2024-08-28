package entities

import (
	"context"
	"fmt"
	"log"
	"main/config"

	"github.com/jackc/pgx/v5"
)

type Application struct {
	id   int
	name string
}

type Inventory struct {
	id   int
	name string
	path string
}

type API struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	ReqType     string      `json:"reqType"`
	Description string      `json:"description"`
	Parameters  string      `json:"parameters"`
	CreatedDate uint64      `json:"createdDate"`
	Inventory   int         `json:"inventory"`
	Responses   string      `json:"responses"`
	OperationID interface{} `json:"operationId"`
	Produces    interface{} `json:"produces"`
	IsNew       bool        `json:"isNew"`
	TestResults string      `json:"testResults"`
}

func GetAllAPIs() ([]API, error) {
	// Query for API data
	rows, err := config.Conn.Query(context.Background(), `
        SELECT id, name, description, path, created_time, req_type, inventory, 
               responses, operationid, produces, is_new 
        FROM api
    `)
	if err != nil {
		return nil, fmt.Errorf("error querying APIs: %w", err)
	}
	defer rows.Close()

	var apis []API
	for rows.Next() {
		var api API
		err = rows.Scan(
			&api.ID, &api.Name, &api.Description, &api.Path, &api.CreatedDate,
			&api.ReqType, &api.Inventory, &api.Responses, &api.OperationID,
			&api.Produces, &api.IsNew,
		)
		if err != nil {
			log.Printf("Error scanning API row: %v", err)
			continue // Skip to next row on error
		}
		apis = append(apis, api)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API rows: %w", err)
	}

	// Query for test results
	for i := range apis {
		err := config.Conn.QueryRow(context.Background(), `
            SELECT results 
            FROM test_results 
            WHERE inventory = $1 AND api_id = $2
        `, apis[i].Inventory, apis[i].ID).Scan(&apis[i].TestResults)

		if err != nil {
			if err == pgx.ErrNoRows {
				log.Printf("No test results found for API %d", apis[i].ID)
			} else {
				log.Printf("Error querying test results for API %d: %v", apis[i].ID, err)
			}
			// Continue to next API even if there's an error
		} else {
			log.Printf("Test results for API %d: %s", apis[i].ID, apis[i].TestResults)
		}
	}

	return apis, nil
}
