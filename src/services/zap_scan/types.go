package main

type Model struct {
	name   string
	fields string
}

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
	id          int
	name        string
	path        string
	reqType     string
	description string
	parameters  string
	createdDate uint64
	inventory   int
	responses   string
	operationid any
	produces    any
	is_new      bool
}

type Response struct {
	id            int
	response_data string
	status        int
	api_id        int
}
