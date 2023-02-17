#!/bin/bash

cd ./src

tinygo build -o ./example.wasm -target=wasi ./example-wasm