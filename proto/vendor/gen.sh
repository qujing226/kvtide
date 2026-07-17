#!/bin/sh
cd $(dirname "$0")
buf generate
cd ../pbgen
mv daotl/protoc-gen-go-string-consts/* .
rm -rf daotl
