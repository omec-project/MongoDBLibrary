module testapp

go 1.15

replace github.com/omec-project/MongoDBLibrary => ../

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/omec-project/MongoDBLibrary v1.1.4-0.20220825211024-3bacf8295862
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/thakurajayL/drsm v0.0.65-dev.0.20220825211150-dc046641af37
	go.mongodb.org/mongo-driver v1.10.1
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)
