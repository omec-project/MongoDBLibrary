// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"context"
	"log"
	"time"

	"github.com/omec-project/MongoDBLibrary"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Student struct {
	//ID     		primitive.ObjectID 	`bson:"_id,omitempty"`
	Name       string                 `bson:"name,omitempty"`
	Age        int                    `bson:"age,omitempty"`
	Subject    string                 `bson:"subject,omitempty"`
	CreatedAt  time.Time              `bson:"createdAt,omitempty"`
	CustomInfo map[string]interface{} `bson:"customInfo,omitempty"`
}

func iterateChangeStream(routineCtx context.Context, stream *mongo.ChangeStream) {
	log.Println("iterate change stream for timeout")
	defer stream.Close(routineCtx)
	for stream.Next(routineCtx) {
		var data bson.M
		if err := stream.Decode(&data); err != nil {
			panic(err)
		}
		log.Println("iterate stream : ", data)
	}
}

func main() {
	log.Println("dbtestapp started")

	// connect to mongoDB
	MongoDBLibrary.SetMongoDB("sdcore", "mongodb://mongodb:27017")

	_, errVal := MongoDBLibrary.CreateIndex("student", "Name")
	if errVal != nil {
		log.Println("Create index failed on Name field : ", errVal)
	}

	//add document to student collection.
	insertStudentInDB("student", "Osman Amjad", 21)
	//update document in student collection.
	insertStudentInDB("student", "Osman Amjad", 22)
	//fetch document from student db based on index
	student, err := getStudentFromDB("student", "Osman Amjad")
	if err == nil {
		log.Println("Printing student1")
		log.Println(student)
		log.Println(student.Name)
		log.Println(student.Age)
		log.Println(student.CreatedAt)
	} else {
		log.Println("Error getting student: " + err.Error())
	}

	insertStudentInDB("student", "John Smith", 25)

	// test document fetch from student that doesn't exist.
	student, err = getStudentFromDB("student", "Nerf Doodle")
	if err == nil {
		log.Println("Printing student2")
		log.Println(student)
		log.Println(student.Name)
		log.Println(student.Age)
		log.Println(student.CreatedAt)
	} else {
		log.Println("Error getting student: " + err.Error())
	}

	log.Println("starting timeout document")
	database := MongoDBLibrary.Client.Database("sdcore")
	timeoutColl := database.Collection("timeout")
	//create stream to monitor actions on the collection
	timeoutStream, err := timeoutColl.Watch(context.TODO(), mongo.Pipeline{})
	if err != nil {
		panic(err)
	}
	routineCtx, _ := context.WithCancel(context.Background())
	//run routine to get messages from stream
	go iterateChangeStream(routineCtx, timeoutStream)
	//createDocumentWithTimeout("timeout", "yak1", 60, "createdAt")
	//createDocumentWithTimeout("timeout", "yak2", 60, "createdAt")
	ret := MongoDBLibrary.RestfulAPICreateTTLIndex("timeout", 20, "updatedAt")
	if ret {
		log.Println("TTL index create successful")
	} else {
		log.Println("TTL index exists already")
	}

	createDocumentWithCommonTimeout("timeout", "yak1")
	updateDocumentWithCommonTimeout("timeout", "yak1")
	go func() {
		for {
			createDocumentWithCommonTimeout("timeout", "yak2")
			time.Sleep(5 * time.Second)
		}
	}()

	ret = MongoDBLibrary.RestfulAPIDropTTLIndex("timeout", "updatedAt")
	if !ret {
		log.Println("TTL index drop failed")
	}
	ret = MongoDBLibrary.RestfulAPIPatchTTLIndex("timeout", 0, "expireAt")
	if ret {
		log.Println("TTL index patch successful")
	} else {
		log.Println("TTL index patch failed")
	}

	createDocumentWithExpiryTime("timeout", "yak1", 30)
	createDocumentWithExpiryTime("timeout", "yak3", 30)
	updateDocumentWithExpiryTime("timeout", "yak3", 40)
	updateDocumentWithExpiryTime("timeout", "yak1", 50)
	//log.Println("sleeping for 120 seconds")
	//time.Sleep(120 * time.Second)
	//updateDocumentWithTimeout("timeout", "yak1", 200, "createdAt")

	uniqueId := MongoDBLibrary.GetUniqueIdentity("tmsi")
	log.Println(uniqueId)

	uniqueId = MongoDBLibrary.GetUniqueIdentity("amfUeNgapId")
	log.Println(uniqueId)

	uniqueId = MongoDBLibrary.GetUniqueIdentityWithinRange(3, 6)
	log.Println(uniqueId)

	uniqueId = MongoDBLibrary.GetUniqueIdentityWithinRange(3, 6)
	log.Println(uniqueId)

	log.Println("TESTING POOL OF IDS")

	MongoDBLibrary.InitializePool("pool1", 10, 32)

	uniqueId, err = MongoDBLibrary.GetIDFromPool("pool1")
	log.Println(uniqueId)

	MongoDBLibrary.ReleaseIDToPool("pool1", uniqueId)

	uniqueId, err = MongoDBLibrary.GetIDFromPool("pool1")
	log.Println(uniqueId)

	uniqueId, err = MongoDBLibrary.GetIDFromPool("pool1")
	log.Println(uniqueId)

	log.Println("TESTING INSERT APPROACH")
	var randomId int32

	randomId, err = MongoDBLibrary.GetIDFromInsertPool("insertApproach")
	log.Println(randomId)
	if err != nil {
		log.Println(err.Error())
	}

	MongoDBLibrary.InitializeInsertPool("insertApproach", 0, 1000, 3)

	randomId, err = MongoDBLibrary.GetIDFromInsertPool("insertApproach")
	log.Println(randomId)
	if err != nil {
		log.Println(err.Error())
	}

	randomId, err = MongoDBLibrary.GetIDFromInsertPool("insertApproach")
	log.Println(randomId)
	if err != nil {
		log.Println(err.Error())
	}

	MongoDBLibrary.ReleaseIDToInsertPool("insertApproach", randomId)

	log.Println("TESTING RETRIES")

	MongoDBLibrary.InitializeInsertPool("testRetry", 0, 6, 3)

	randomId, err = MongoDBLibrary.GetIDFromInsertPool("testRetry")
	log.Println(randomId)
	if err != nil {
		log.Println(err.Error())
	}

	randomId, err = MongoDBLibrary.GetIDFromInsertPool("testRetry")
	log.Println(randomId)
	if err != nil {
		log.Println(err.Error())
	}

	log.Println("TESTING CHUNK APPROACH")
	var lower int32
	var upper int32

	randomId, lower, upper, err = MongoDBLibrary.GetChunkFromPool("studentIdsChunkApproach")
	log.Println(randomId, lower, upper)
	if err != nil {
		log.Println(err.Error())
	}

	MongoDBLibrary.InitializeChunkPool("studentIdsChunkApproach", 0, 1000, 5, 100) // min, max, retries, chunkSize

	randomId, lower, upper, err = MongoDBLibrary.GetChunkFromPool("studentIdsChunkApproach")
	log.Println(randomId, lower, upper)
	if err != nil {
		log.Println(err.Error())
	}

	randomId, lower, upper, err = MongoDBLibrary.GetChunkFromPool("studentIdsChunkApproach")
	log.Println(randomId, lower, upper)
	if err != nil {
		log.Println(err.Error())
	}

	randomId, lower, upper, err = MongoDBLibrary.GetChunkFromPool("studentIdsChunkApproach")
	log.Println(randomId, lower, upper)
	if err != nil {
		log.Println(err.Error())
	}

	MongoDBLibrary.ReleaseChunkToPool("studentIdsChunkApproach", randomId)

	for {
		time.Sleep(100 * time.Second)
	}
}

func getStudentFromDB(collName string, name string) (Student, error) {
	var student Student
	filter := bson.M{}
	filter["name"] = name

	result, err := MongoDBLibrary.GetOneCustomDataStructure(collName, filter)

	if err == nil {
		bsonBytes, _ := bson.Marshal(result)
		bson.Unmarshal(bsonBytes, &student)

		return student, nil
	}
	return student, err
}

func insertStudentInDB(collName string, name string, age int) {
	student := Student{
		Name:      name,
		Age:       age,
		CreatedAt: time.Now(),
	}
	filter := bson.M{}
	_, err := MongoDBLibrary.PutOneCustomDataStructure(collName, filter, student)
	if err != nil {
		log.Println("put data failed : ", err)
		return
	}
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

func updateDocumentWithExpiryTime(collName string, name string, timeVal int) {
	putData := bson.M{}
	putData["name"] = name
	//putData["createdAt"] = time.Now()
	timein := time.Now().Local().Add(time.Second * time.Duration(timeVal))
	putData["expireAt"] = timein
	filter := bson.M{"name": name}
	MongoDBLibrary.RestfulAPIPutOne(collName, filter, putData)
}

func createDocumentWithCommonTimeout(collName string, name string) {
	putData := bson.M{}
	putData["name"] = name
	putData["createdAt"] = time.Now()
	//timein := time.Now().Local().Add(time.Second * time.Duration(20))
	//log.Println("updated timeout : ", timein)
	//putData["updatedAt"] = timein
	putData["updatedAt"] = time.Now()
	filter := bson.M{"name": name}
	MongoDBLibrary.RestfulAPIPutOne(collName, filter, putData)
}

func updateDocumentWithCommonTimeout(collName string, name string) {
	putData := bson.M{}
	putData["name"] = name
	//putData["createdAt"] = time.Now()
	putData["updatedAt"] = time.Now()
	filter := bson.M{"name": name}
	MongoDBLibrary.RestfulAPIPutOne("timeout", filter, putData)
}
