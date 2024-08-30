#!/bin/bash

# start backend
cd backend
go run . &

# start frontend
cd ../frontend
npm run dev