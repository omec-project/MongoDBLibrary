// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package MongoDBLibrary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"

	"github.com/omec-project/MongoDBLibrary/logger"
)

var Client *mongo.Client = nil
var dbName string
var pools = map[string]map[string]int{}

func SetMongoDB(setdbName string, url string) {

	if Client != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(url))
	defer cancel()
	if err != nil {
		//defer cancel()
		logger.MongoDBLog.Println("SetMongoDB: Mongo connect failed for url (", url, ") : ", err.Error())
	}
	Client = client
	dbName = setdbName
}

func RestfulAPIGetOne(collName string, filter bson.M) map[string]interface{} {

	collection := Client.Database(dbName).Collection(collName)

	var result map[string]interface{}
	collection.FindOne(context.TODO(), filter).Decode(&result)

	return result
}

func RestfulAPIGetMany(collName string, filter bson.M) []map[string]interface{} {
	collection := Client.Database(dbName).Collection(collName)

	var resultArray []map[string]interface{}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cur, err := collection.Find(ctx, filter)
	defer cancel()
	if err != nil {
		logger.MongoDBLog.Println("RestfulAPIGetMany : DB fetch failed for collection (", collName, ") : ", err)
		return nil
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var result map[string]interface{}
		err := cur.Decode(&result)
		if err != nil {
			logger.MongoDBLog.Println("RestfulAPIGetMany : Cursor decode failed for collection (", collName, ") : ", err)
		}
		resultArray = append(resultArray, result)
	}
	if err := cur.Err(); err != nil {
		logger.MongoDBLog.Println("RestfulAPIGetMany : Cursor read error for collection (", collName, ") : ", err)
	}

	return resultArray

}

/* Get unique identity from counter collection. */
func GetUniqueIdentity(idName string) int32 {
	counterCollection := Client.Database(dbName).Collection("counter")

	counterFilter := bson.M{}
	counterFilter["_id"] = idName

	for {
		count := counterCollection.FindOneAndUpdate(context.TODO(), counterFilter, bson.M{"$inc": bson.M{"count": 1}})

		if count.Err() != nil {
			logger.MongoDBLog.Println("FindOneAndUpdate error. Create entry for field  ")
			counterData := bson.M{}
			counterData["count"] = 1
			counterData["_id"] = idName
			counterCollection.InsertOne(context.TODO(), counterData)

			continue
		} else {
			logger.MongoDBLog.Println("found entry. inc and return")
			data := bson.M{}
			count.Decode(&data)
			decodedCount := data["count"].(int32)
			return decodedCount
		}
	}
}

/* Get a unique id within a given range. */
func GetUniqueIdentityWithinRange(min int32, max int32) int32 {
	rangeCollection := Client.Database(dbName).Collection("range")

	rangeFilter := bson.M{}
	rangeFilter["_id"] = "uniqueIdentity"

	for {
		count := rangeCollection.FindOneAndUpdate(context.TODO(), rangeFilter, bson.M{"$inc": bson.M{"count": 1}})

		if count.Err() != nil {
			counterData := bson.M{}
			counterData["count"] = min
			counterData["_id"] = "uniqueIdentity"
			rangeCollection.InsertOne(context.TODO(), counterData)

			continue
		} else {
			data := bson.M{}
			count.Decode(&data)
			decodedCount := data["count"].(int32)

			if decodedCount >= max || decodedCount <= min {
				err := errors.New("Unique identity is out of range.")
				logger.MongoDBLog.Println(err)
				return -1
			}
			return decodedCount
		}
	}
}

/* Initialize pool of ids with max and min values and chunk size and amount of retries to get a chunk. */
func InitializeChunkPool(poolName string, min int, max int, retries int, chunkSize int) {
	logger.MongoDBLog.Println("ENTERING InitializeChunkPool")
	var poolData = map[string]int{}
	poolData["min"] = min
	poolData["max"] = max
	poolData["retries"] = retries
	poolData["chunkSize"] = chunkSize

	pools[poolName] = poolData
	logger.MongoDBLog.Println("Pools: ", pools)
}

/* Get id by inserting into collection. If insert succeeds, that id is available. Else, it isn't available so retry. */
func GetChunkFromPool(poolName string) (int32, int32, int32, error) {
	logger.MongoDBLog.Println("ENTERING GetChunkFromPool")

	var pool = pools[poolName]

	if pool == nil {
		err := errors.New("This pool has not been initialized yet. Initialize by calling InitializeChunkPool.")
		return -1, -1, -1, err
	}

	min := pool["min"]
	max := pool["max"]
	retries := pool["retries"]
	chunkSize := pool["chunkSize"]
	totalChunks := int((max - min) / chunkSize)

	i := 0
	for i < retries {
		random := rand.Intn(totalChunks)
		lower := min + (random * chunkSize)
		upper := lower + chunkSize
		poolCollection := Client.Database(dbName).Collection(poolName)

		// Create an instance of an options and set the desired options
		upsert := true
		opt := options.FindOneAndUpdateOptions{
			Upsert: &upsert,
		}
		data := bson.M{}
		data["_id"] = random
		data["lower"] = lower
		data["upper"] = upper
		data["owner"] = os.Getenv("HOSTNAME")
		result := poolCollection.FindOneAndUpdate(context.TODO(), bson.M{"_id": random}, bson.M{"$setOnInsert": data}, &opt)

		if result.Err() != nil {
			// means that there was no document with that id, so the upsert should have been successful
			if result.Err() == mongo.ErrNoDocuments {
				logger.MongoDBLog.Println("Assigned chunk # ", random, " with range ", lower, " - ", upper)
				return int32(random), int32(lower), int32(upper), nil
			}

			return -1, -1, -1, result.Err()
		}
		// means there was a document before the update and result contains that document.
		logger.MongoDBLog.Println("Chunk", random, " has already been assigned. ", retries-i-1, " retries left.")
		i++
	}

	err := errors.New("No id found after retries")
	return -1, -1, -1, err
}

/* Release the provided id to the provided pool. */
func ReleaseChunkToPool(poolName string, id int32) {
	logger.MongoDBLog.Println("ENTERING ReleaseChunkToPool")
	poolCollection := Client.Database(dbName).Collection(poolName)

	// only want to delete if the currentApp is the owner of this id.
	currentApp := os.Getenv("HOSTNAME")
	logger.MongoDBLog.Println(currentApp)

	_, err := poolCollection.DeleteOne(context.TODO(), bson.M{"_id": id, "owner": currentApp})
	if err != nil {
		logger.MongoDBLog.Println("Release Chunk(", id, ") to Pool(", poolName, ") failed : ", err)
	}
}

/* Initialize pool of ids with max and min values. */
func InitializeInsertPool(poolName string, min int, max int, retries int) {
	logger.MongoDBLog.Println("ENTERING InitializeInsertPool")
	var poolData = map[string]int{}
	poolData["min"] = min
	poolData["max"] = max
	poolData["retries"] = retries

	pools[poolName] = poolData
	logger.MongoDBLog.Println("Pools: ", pools)
}

/* Get id by inserting into collection. If insert succeeds, that id is available. Else, it isn't available so retry. */
func GetIDFromInsertPool(poolName string) (int32, error) {
	logger.MongoDBLog.Println("ENTERING GetIDFromInsertPool")

	var pool = pools[poolName]

	if pool == nil {
		err := errors.New("This pool has not been initialized yet. Initialize by calling InitializeInsertPool.")
		return -1, err
	}

	min := pool["min"]
	max := pool["max"]
	retries := pool["retries"]
	i := 0
	for i < retries {
		random := rand.Intn(max-min) + min // returns random int in [0, max-min-1] + min
		poolCollection := Client.Database(dbName).Collection(poolName)

		// Create an instance of an options and set the desired options
		upsert := true
		opt := options.FindOneAndUpdateOptions{
			Upsert: &upsert,
		}
		result := poolCollection.FindOneAndUpdate(context.TODO(), bson.M{"_id": random}, bson.M{"$set": bson.M{"_id": random}}, &opt)

		if result.Err() != nil {
			// means that there was no document with that id, so the upsert should have been successful
			if result.Err().Error() == "mongo: no documents in result" {
				logger.MongoDBLog.Println("Assigned id: ", random)
				return int32(random), nil
			}

			return -1, result.Err()
		}
		// means there was a document before the update and result contains that document.
		logger.MongoDBLog.Println("This id has already been assigned. ")
		doc := bson.M{}
		result.Decode(&doc)
		logger.MongoDBLog.Println(doc)

		i++
	}

	err := errors.New("No id found after retries")
	return -1, err
}

/* Release the provided id to the provided pool. */
func ReleaseIDToInsertPool(poolName string, id int32) {
	logger.MongoDBLog.Println("ENTERING ReleaseIDToInsertPool")
	poolCollection := Client.Database(dbName).Collection(poolName)

	_, err := poolCollection.DeleteOne(context.TODO(), bson.M{"_id": id})
	if err != nil {
		logger.MongoDBLog.Println("Release Id(", id, ") to Pool(", poolName, ") failed : ", err)
	}
}

/* Initialize pool of ids with max and min values. */
func InitializePool(poolName string, min int32, max int32) {
	logger.MongoDBLog.Println("ENTERING InitializePool")
	poolCollection := Client.Database(dbName).Collection(poolName)
	names, err := Client.Database(dbName).ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		logger.MongoDBLog.Println(err)
		return
	}

	logger.MongoDBLog.Println(names)

	exists := false
	for _, name := range names {
		if name == poolName {
			logger.MongoDBLog.Println("The collection exists!")
			exists = true
			break
		}
	}
	if !exists {
		logger.MongoDBLog.Println("Creating collection")

		array := []int32{}
		for i := min; i < max; i++ {
			array = append(array, i)
		}
		poolData := bson.M{}
		poolData["ids"] = array
		poolData["_id"] = poolName

		// collection is created when inserting document.
		// "If a collection does not exist, MongoDB creates the collection when you first store data for that collection."
		poolCollection.InsertOne(context.TODO(), poolData)
	}
}

/* For example IP addresses need to be assigned and then returned to be used again. */
func GetIDFromPool(poolName string) (int32, error) {
	logger.MongoDBLog.Println("ENTERING GetIDFromPool")
	poolCollection := Client.Database(dbName).Collection(poolName)

	result := bson.M{}
	poolCollection.FindOneAndUpdate(context.TODO(), bson.M{"_id": poolName}, bson.M{"$pop": bson.M{"ids": 1}}).Decode(&result)

	var array []int32
	interfaces := []interface{}(result["ids"].(primitive.A))
	for _, s := range interfaces {
		id := s.(int32)
		array = append(array, id)
	}

	logger.MongoDBLog.Println("Array of ids: ", array)
	if len(array) > 0 {
		res := array[len(array)-1]
		return res, nil
	} else {
		err := errors.New("There are no available ids.")
		logger.MongoDBLog.Println(err)
		return -1, err
	}
}

/* Release the provided id to the provided pool. */
func ReleaseIDToPool(poolName string, id int32) {
	logger.MongoDBLog.Println("ENTERING ReleaseIDToPool")
	poolCollection := Client.Database(dbName).Collection(poolName)

	poolCollection.UpdateOne(context.TODO(), bson.M{"_id": poolName}, bson.M{"$push": bson.M{"ids": id}})
}

func GetOneCustomDataStructure(collName string, filter bson.M) (bson.M, error) {
	collection := Client.Database(dbName).Collection(collName)

	val := collection.FindOne(context.TODO(), filter)

	if val.Err() != nil {
		logger.MongoDBLog.Println("Error getting student from db: " + val.Err().Error())
		return bson.M{}, val.Err()
	}

	var result bson.M
	err := val.Decode(&result)
	return result, err
}

func PutOneCustomDataStructure(collName string, filter bson.M, putData interface{}) (bool, error) {
	collection := Client.Database(dbName).Collection(collName)

	var checkItem map[string]interface{}
	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		_, err := collection.InsertOne(context.TODO(), putData)
		if err != nil {
			logger.MongoDBLog.Println("insert failed : ", err)
			return false, err
		}
		return true, nil
	} else {
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		return true, nil
	}
}

func CreateIndex(collName string, keyField string) (bool, error) {
	collection := Client.Database(dbName).Collection(collName)

	index := mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: keyField, Value: bsonx.Int32(1)}},
		Options: options.Index().SetUnique(true),
	}

	idx, err := collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		logger.MongoDBLog.Error("Create Index failed : ", keyField, err)
		return false, err
	}

	logger.MongoDBLog.Println("Created index : ", idx, " on keyField : ", keyField, " for Collection : ", collName)

	return true, nil
}

// To create Index with common timeout for all documents, set timeout to desired value
// To create Index with custom timeout per document, set timeout to 0.
// To create Index with common timeout use timefield name like : updatedAt
// To create Index with custom timeout use timefield name like : expireAt
func RestfulAPICreateTTLIndex(collName string, timeout int32, timeField string) bool {
	collection := Client.Database(dbName).Collection(collName)
	index := mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: timeField, Value: bsonx.Int32(1)}},
		Options: options.Index().SetExpireAfterSeconds(timeout).SetName(timeField),
	}

	_, err := collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		logger.MongoDBLog.Println("Index creation failed for Field : ", timeField, " in Collection : ", collName, " Error Cause : ", err)
		return false
	}

	return true
}

// Use this API to drop TTL Index.
func RestfulAPIDropTTLIndex(collName string, timeField string) bool {
	collection := Client.Database(dbName).Collection(collName)
	_, err := collection.Indexes().DropOne(context.Background(), timeField)
	if err != nil {
		logger.MongoDBLog.Println("Drop Index on field (", timeField, ") for collection (", collName, ") failed : ", err)
		return false
	}

	return true
}

// Use this API to update timeout value for TTL Index.
func RestfulAPIPatchTTLIndex(collName string, timeout int32, timeField string) bool {
	collection := Client.Database(dbName).Collection(collName)
	_, err := collection.Indexes().DropOne(context.Background(), timeField)
	if err != nil {
		logger.MongoDBLog.Println("Drop Index on field (", timeField, ") for collection (", collName, ") failed : ", err)
	}

	//create new index with new timeout
	index := mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: timeField, Value: bsonx.Int32(1)}},
		Options: options.Index().SetExpireAfterSeconds(timeout).SetName(timeField),
	}

	_, err = collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		logger.MongoDBLog.Println("Index on field (", timeField, ") for collection (", collName, ") already exists : ", err)
	}

	return true
}

// This API adds document to collection with name : "collName"
// This API should be used when we wish to update the timeout value for the TTL index
// It checks if an Index with name "indexName" exists on the collection.
// If such an Index is "indexName" is found, we drop the index and then
// add new Index with new timeout value.
func RestfulAPIPatchOneTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool {
	collection := Client.Database(dbName).Collection(collName)
	var checkItem map[string]interface{}

	//fetch all Indexes on collection
	cursor, err := collection.Indexes().List(context.TODO())

	if err != nil {
		logger.MongoDBLog.Println("RestfulAPIPatchOneTimeout : List Index failed for collection (", collName, ") : ", err)
		return false
	}

	var result []bson.M
	// convert to map
	if err = cursor.All(context.TODO(), &result); err != nil {
		logger.MongoDBLog.Println("RestfulAPIPatchOneTimeout : Cursor decode failed for collection (", collName, ") : ", err)
	}

	// loop through the map and check for entry with key as name
	// for every entry with key as name,check if the value string contains the timeField string.
	// the Indexes are generally named such as follows :
	// field name : createdAt, index name : createdAt_1
	// drop the index if found.
	drop := false
	for _, v := range result {
		for k1, v1 := range v {
			valStr := fmt.Sprint(v1)
			if (k1 == "name") && strings.Contains(valStr, timeField) {
				_, err = collection.Indexes().DropOne(context.Background(), valStr)
				if err != nil {
					logger.MongoDBLog.Println("Drop Index on field (", timeField, ") for collection (", collName, ") failed : ", err)
					break
				}
			}
		}
		if drop {
			break
		}
	}

	//create new index with new timeout
	index := mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: timeField, Value: bsonx.Int32(1)}},
		Options: options.Index().SetExpireAfterSeconds(timeout),
	}

	_, err = collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		logger.MongoDBLog.Println("Index on field (", timeField, ") for collection (", collName, ") already exists : ", err)
	}

	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		collection.InsertOne(context.TODO(), putData)
		return false
	} else {
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		return true
	}
}

// This API adds document to collection with name : "collName"
// It also creates an Index with Time to live (TTL) on the collection.
// All Documents in the collection will have the the same TTL. The timestamps
// each document can be different and can be updated as per procedure.
// If the Index with same timeout value is present already then it
// does not create a new one.
// If the Index exists on the same "timeField" with a different timeout,
// then API will return error saying Index already exists.
func RestfulAPIPutOneTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool {
	collection := Client.Database(dbName).Collection(collName)
	var checkItem map[string]interface{}

	/*index := mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: timeField, Value: bsonx.Int32(1)}},
		Options: options.Index().SetExpireAfterSeconds(timeout),
	}

	_, err := collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		logger.MongoDBLog.Println("Index creation failed for Field : ", timeField, " in Collection : ", collName)
	}*/

	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		collection.InsertOne(context.TODO(), putData)
		return false
	} else {
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		return true
	}
}

func RestfulAPIPutOne(collName string, filter bson.M, putData map[string]interface{}) bool {
	collection := Client.Database(dbName).Collection(collName)

	var checkItem map[string]interface{}
	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		collection.InsertOne(context.TODO(), putData)
		return false
	} else {
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		return true
	}
}

func RestfulAPIPutOneNotUpdate(collName string, filter bson.M, putData map[string]interface{}) bool {
	collection := Client.Database(dbName).Collection(collName)

	var checkItem map[string]interface{}
	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		collection.InsertOne(context.TODO(), putData)
		return false
	} else {
		// collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		return true
	}
}

func RestfulAPIPutMany(collName string, filterArray []bson.M, putDataArray []map[string]interface{}) bool {
	collection := Client.Database(dbName).Collection(collName)

	var checkItem map[string]interface{}
	for i, putData := range putDataArray {
		checkItem = nil
		filter := filterArray[i]
		collection.FindOne(context.TODO(), filter).Decode(&checkItem)

		if checkItem == nil {
			collection.InsertOne(context.TODO(), putData)
		} else {
			collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		}
	}

	if checkItem == nil {
		return false
	} else {
		return true
	}

}

func RestfulAPIDeleteOne(collName string, filter bson.M) {
	collection := Client.Database(dbName).Collection(collName)

	collection.DeleteOne(context.TODO(), filter)
}

func RestfulAPIDeleteMany(collName string, filter bson.M) {
	collection := Client.Database(dbName).Collection(collName)

	collection.DeleteMany(context.TODO(), filter)
}

func RestfulAPIMergePatch(collName string, filter bson.M, patchData map[string]interface{}) bool {
	collection := Client.Database(dbName).Collection(collName)

	var originalData map[string]interface{}
	result := collection.FindOne(context.TODO(), filter)

	if err := result.Decode(&originalData); err != nil { // Data doesn't exist in DB
		return false
	} else {
		delete(originalData, "_id")
		original, _ := json.Marshal(originalData)

		patchDataByte, err := json.Marshal(patchData)
		if err != nil {
			logger.MongoDBLog.Println("RestfulAPIMergePatch: Json Marshal failed for collection (", collName, ") : ", err)
		}

		modifiedAlternative, err := jsonpatch.MergePatch(original, patchDataByte)
		if err != nil {
			logger.MongoDBLog.Println("RestfulAPIMergePatch: Json merge patch failed for collection (", collName, ") : ", err)
		}

		var modifiedData map[string]interface{}

		json.Unmarshal(modifiedAlternative, &modifiedData)
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": modifiedData})
		return true
	}
}

func RestfulAPIJSONPatch(collName string, filter bson.M, patchJSON []byte) bool {
	collection := Client.Database(dbName).Collection(collName)

	var originalData map[string]interface{}
	result := collection.FindOne(context.TODO(), filter)

	if err := result.Decode(&originalData); err != nil { // Data doesn't exist in DB
		return false
	} else {
		delete(originalData, "_id")
		original, _ := json.Marshal(originalData)

		patch, err := jsonpatch.DecodePatch(patchJSON)
		if err != nil {
			logger.MongoDBLog.Println("RestfulAPIJSONPatch: Json decode patch failed for collection (", collName, ") : ", err)
		}

		modified, err := patch.Apply(original)
		if err != nil {
			logger.MongoDBLog.Println("RestfulAPIJSONPatch: Patch apply failed for collection (", collName, ") : ", err)
		}

		var modifiedData map[string]interface{}

		json.Unmarshal(modified, &modifiedData)
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": modifiedData})
		return true
	}

}

func RestfulAPIJSONPatchExtend(collName string, filter bson.M, patchJSON []byte, dataName string) bool {
	collection := Client.Database(dbName).Collection(collName)

	var originalDataCover map[string]interface{}
	result := collection.FindOne(context.TODO(), filter)

	if err := result.Decode(&originalDataCover); err != nil { // Data does'nt exist in db
		return false
	} else {
		delete(originalDataCover, "_id")
		originalData := originalDataCover[dataName]
		original, _ := json.Marshal(originalData)

		jsonpatch.DecodePatch(patchJSON)
		patch, err := jsonpatch.DecodePatch(patchJSON)
		if err != nil {
			logger.MongoDBLog.Println("RestfulAPIJSONPatchExtend: Patch decode failed for collection (", collName, ") : ", err)
		}

		modified, err := patch.Apply(original)
		if err != nil {
			logger.MongoDBLog.Println("RestfulAPIJSONPatchExtend: Patch apply failed for collection (", collName, ") : ", err)
		}

		var modifiedData map[string]interface{}
		json.Unmarshal(modified, &modifiedData)
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": bson.M{dataName: modifiedData}})
		return true
	}
}

func RestfulAPIPost(collName string, filter bson.M, postData map[string]interface{}) bool {
	collection := Client.Database(dbName).Collection(collName)

	var checkItem map[string]interface{}
	err := collection.FindOne(context.TODO(), filter).Decode(&checkItem)
	if err != nil {
		logger.MongoDBLog.Println("item not found: ", err)
	}

	if checkItem == nil {
		_, err := collection.InsertOne(context.TODO(), postData)
		if err != nil {
			logger.MongoDBLog.Println("insert failed : ", err)
			return false
		}
		return false
	} else {
		_, err := collection.UpdateOne(context.TODO(), filter, bson.M{"$set": postData})
		if err != nil {
			logger.MongoDBLog.Println("update failed : ", err)
			return false
		}
		return true
	}
}

func RestfulAPIPostMany(collName string, filter bson.M, postDataArray []interface{}) bool {
	collection := Client.Database(dbName).Collection(collName)

	collection.InsertMany(context.TODO(), postDataArray)
	return false
}
