#!/bin/bash

cd ./src

GOOS=js GOARCH=wasm go build -o ./example.wasm ./example-wasm
