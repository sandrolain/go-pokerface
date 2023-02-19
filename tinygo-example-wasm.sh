#!/bin/bash

cd ./src

tinygo build -o ./example.wasm -no-debug -gc=leaking -scheduler=none -target=wasi ./example-wasm