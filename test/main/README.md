# test for nomad
build
```
go build test_main.go
tar czf test.tar.gz test_main
```
deploy
```
levant deploy -var-file=test.yml test.nomad
```

support nomad rolling update 

```
levant plan -var-file=test1.yml test.nomad

levant deploy -var-file=test1.yml test.nomad
```
run as client(use fabio as elb proxy)
```
./test_main -cli=true -addr=127.0.0.1:8888 -cnt=100 -print=false
```