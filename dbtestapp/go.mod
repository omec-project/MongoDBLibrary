module testapp

go 1.15

replace github.com/omec-project/MongoDBLibrary => ../

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/omec-project/MongoDBLibrary v1.1.3
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/thakurajayL/drsm v0.0.65-dev.0.20220825204158-2ec46f36d550
	go.mongodb.org/mongo-driver v1.10.1
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)
