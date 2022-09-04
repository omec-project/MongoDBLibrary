// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"log"
	"time"
	"go.mongodb.org/mongo-driver/bson"
	"github.com/omec-project/MongoDBLibrary"
)


// TODO : take DB name from helm chart 
// TODO : inbuild shell commands to  

func main() {
	log.Println("dbtestapp started")
	// connect to mongoDB
	MongoDBLibrary.SetMongoDB("sdcore", "mongodb://mongodb-arbiter-headless")

    initDrsm("ngapid")

	http_server()
}

func deleteDocumentWithTimeout(name string) {
	putData := bson.M{}
	putData["name"] = name
	filter := bson.M{}
	MongoDBLibrary.RestfulAPIDeleteOne("timeout", filter)
}

func createDocumentWithExpiryTime(collName string, name string, timeVal int) {
	putData := bson.M{}
	putData["name"] = name
	putData["createdAt"] = time.Now()
	timein := time.Now().Local().Add(time.Second * time.Duration(timeVal))
	//log.Println("updated timeout : ", timein)
	putData["expireAt"] = timein
	//putData["updatedAt"] = time.Now()
	filter := bson.M{"name": name}
	MongoDBLibrary.RestfulAPIPutOne(collName, filter, putData)
}



