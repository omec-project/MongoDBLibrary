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
	"math/rand"
	"os"
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
		logger.MongoDBLog.Panic(err.Error())
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
		logger.MongoDBLog.Fatal(err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var result map[string]interface{}
		err := cur.Decode(&result)
		if err != nil {
			logger.MongoDBLog.Fatal(err)
		}
		resultArray = append(resultArray, result)
	}
	if err := cur.Err(); err != nil {
		logger.MongoDBLog.Fatal(err)
	}

	return resultArray

}

/* Get unique identity from counter collection. */
func GetUniqueIdentity() int32 {
	counterCollection := Client.Database(dbName).Collection("counter")

	counterFilter := bson.M{}
	counterFilter["_id"] = "uniqueIdentity"

	for {
		count := counterCollection.FindOneAndUpdate(context.TODO(), counterFilter, bson.M{"$inc": bson.M{"count": 1}})

		if count.Err() != nil {
			counterData := bson.M{}
			counterData["count"] = 1
			counterData["_id"] = "uniqueIdentity"
			counterCollection.InsertOne(context.TODO(), counterData)

			continue
		} else {
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
		logger.MongoDBLog.Panic(err)
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
		logger.MongoDBLog.Panic(err)
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

func PutOneCustomDataStructure(collName string, filter bson.M, putData interface{}) bool {
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

func PutOneWithTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool {
	collection := Client.Database(dbName).Collection(collName)
	var checkItem map[string]interface{}

	// TTL index
	index := mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: timeField, Value: bsonx.Int32(1)}},
		Options: options.Index().SetExpireAfterSeconds(timeout),
	}

	_, err := collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		logger.MongoDBLog.Panic(err)
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
			logger.MongoDBLog.Panic(err)
		}

		modifiedAlternative, err := jsonpatch.MergePatch(original, patchDataByte)
		if err != nil {
			logger.MongoDBLog.Panic(err)
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
			logger.MongoDBLog.Panic(err)
		}

		modified, err := patch.Apply(original)
		if err != nil {
			logger.MongoDBLog.Panic(err)
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
			logger.MongoDBLog.Panic(err)
		}

		modified, err := patch.Apply(original)
		if err != nil {
			logger.MongoDBLog.Panic(err)
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
	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		collection.InsertOne(context.TODO(), postData)
		return false
	} else {
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": postData})
		return true
	}
}

func RestfulAPIPostMany(collName string, filter bson.M, postDataArray []interface{}) bool {
	collection := Client.Database(dbName).Collection(collName)

	collection.InsertMany(context.TODO(), postDataArray)
	return false
}
