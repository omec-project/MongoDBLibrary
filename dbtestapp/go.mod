module testapp

go 1.15

replace github.com/omec-project/MongoDBLibrary => ../

require (
	github.com/omec-project/MongoDBLibrary v1.1.4-0.20220825211024-3bacf8295862
	github.com/thakurajayL/drsm v0.0.67-dev
	go.mongodb.org/mongo-driver v1.10.1
)
