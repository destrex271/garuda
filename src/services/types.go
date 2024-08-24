package main

type Model struct{
    name string
    fields string
}

type API struct{
    id int
    name string
    description string
    path string
    parameters string
    createdDate uint64
}

type Response struct{
    id int
    response_data string
    status int
    api_id int
}
