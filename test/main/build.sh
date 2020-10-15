#!/bin/sh

go build test_main.go
tar czf test.tar.gz test_main
cp test.tar.gz ~/nginx.d/
cp ~/nginx.d/test.tar.gz ~/nginx.d/test1.tar.gz
