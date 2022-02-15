package MongoDBLibrary

import (
	"context"
	"encoding/json"
	"time"
	"errors"

	jsonpatch "github.com/evanphx/json-patch"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/free5gc/MongoDBLibrary/logger"
)

var Client *mongo.Client = nil
var dbName string

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

			if (decodedCount >= max || decodedCount <= min) {
				err := errors.New("Unique identity is out of range.")
				logger.MongoDBLog.Println(err)
				return -1
			}
			return decodedCount
		}
	}
}

/* Initialize pool of ids with max and min values. */
func InitializePool(poolName string, min int32, max int32) {
	poolCollection := Client.Database(dbName).Collection(poolName)
	array := []int32{}
	for i := min; i < max; i++ {
		array = append(array, i)
	}
	poolData := bson.M{}
	poolData["ids"] = array
	poolData["_id"] = "1"

	poolCollection.InsertOne(context.TODO(), poolData)
}

/* For example IP addresses need to be assigned and then returned to be used again. */
// need to test with empty array.
func GetIDFromPool(poolName string) (int32, error) {
	poolCollection := Client.Database(dbName).Collection(poolName)

	poolFilter := bson.M{}
	poolFilter["_id"] = "1"

	result := bson.M{}
	poolCollection.FindOne(context.TODO(), poolFilter).Decode(&result)
	logger.MongoDBLog.Println(result["ids"])

	//var pool Pool
	var array []int32
	//prim := result["ids"].(primitive.A)
	//logger.MongoDBLog.Println(prim)
	interfaces := []interface{}(result["ids"].(primitive.A))
	logger.MongoDBLog.Println(interfaces)
	for _, s := range interfaces {
		id := s.(int32)
		array = append(array, id)
	}

	logger.MongoDBLog.Println(array)
	if len(array) != 0 {
		res := array[len(array) - 1]
		// pop from array
		updateFilter := bson.M{}
		updateFilter["_id"] = "1"
		poolCollection.UpdateOne(context.TODO(), bson.M{"_id": "1"}, bson.M{"$pop": bson.M{"ids":1}} )
		return res, nil
	} else {
		err := errors.New("There are no available ids. ")
		logger.MongoDBLog.Println(err)
		return -1, err
	}
}

/* Release the provided id from the provided pool. */
func ReleaseIDFromPool(poolName string, id int32) {
	poolCollection := Client.Database(dbName).Collection(poolName)

	poolCollection.UpdateOne(context.TODO(), bson.M{"_id": "1"}, bson.M{"$push": bson.M{"ids":id}})
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

	var checkItem map[string] interface{}
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
