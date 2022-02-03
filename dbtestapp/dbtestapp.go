// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"log"
	"time"

	//"context"
	//"fmt"
	//"os"

	//"go.mongodb.org/mongo-driver/bson/primitive"
	//"go.mongodb.org/mongo-driver/mongo"
	//"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/omec-project/MongoDBLibrary"
	"go.mongodb.org/mongo-driver/bson"
)

type Student struct {
	//ID     		primitive.ObjectID 	`bson:"_id,omitempty"`
	Name      string    `bson:"name,omitempty"`
	Age       int       `bson:"age,omitempty"`
	CreatedAt time.Time `bson:"createdAt,omitempty"`
}

func main() {
	log.Println("dbtestapp started")

	// connect to mongoDB
	MongoDBLibrary.SetMongoDB("free5gc", "mongodb://mongodb:27017")

	insertStudentInDB("Osman Amjad", 21)
	student, err := getStudentFromDB("Osman Amjad")
	if err == nil {
		log.Println("Printing student1")
		log.Println(student)
		log.Println(student.Name)
		log.Println(student.Age)
		log.Println(student.CreatedAt)
	} else {
		log.Println("Error getting student: " + err.Error())
	}

	insertStudentInDB("John Smith", 25)

	// test student that doesn't exist.
	student, err = getStudentFromDB("Nerf Doodle")
	if err == nil {
		log.Println("Printing student2")
		log.Println(student)
		log.Println(student.Name)
		log.Println(student.Age)
		log.Println(student.CreatedAt)
	} else {
		log.Println("Error getting student: " + err.Error())
	}

	createDocumentWithTimeout()

	uniqueId := MongoDBLibrary.GetUniqueIdentity()
	log.Println(uniqueId)

	uniqueId = MongoDBLibrary.GetUniqueIdentity()
	log.Println(uniqueId)

	uniqueId = MongoDBLibrary.GetUniqueIdentity()
	log.Println(uniqueId)

	for {
		time.Sleep(100 * time.Second)
	}
}

func getStudentFromDB(name string) (Student, error) {
	var student Student
	filter := bson.M{}
	filter["name"] = name

	result, err := MongoDBLibrary.GetOneCustomDataStructure("student", filter)

	if err == nil {
		bsonBytes, _ := bson.Marshal(result)
		bson.Unmarshal(bsonBytes, &student)

		return student, nil
	}
	return student, err
}

func insertStudentInDB(name string, age int) {
	student := Student{
		Name:      name,
		Age:       age,
		CreatedAt: time.Now(),
	}
	filter := bson.M{}
	MongoDBLibrary.PutOneCustomDataStructure("student", filter, student)
}

func createDocumentWithTimeout() {
	putData := bson.M{}
	putData["name"] = "Yak"
	putData["createdAt"] = time.Now()
	filter := bson.M{}
	MongoDBLibrary.PutOneWithTimeout("timeout", filter, putData, 120, "createdAt")
}
