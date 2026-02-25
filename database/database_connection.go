package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var dbClient *mongo.Client

func Connect() *mongo.Client {
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}
	uri := os.Getenv("MONGODB_URI")
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)
	client, err := mongo.Connect(opts)
	if err != nil {
		panic(err)
	}
	// defer func() {
	// 	if err = client.Disconnect(context.TODO()); err != nil {
	// 		panic(err)
	// 	}
	// }()
	// Send a ping to confirm a successful connection
	if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
		panic(err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
	return client
}

func OpenCollection(collectionName string) *mongo.Collection {
	dbClient = Connect()
	databaseName := os.Getenv("DATABASE_NAME")
	fmt.Println("DATABASE_NAME: ", databaseName)
	collection := dbClient.Database(databaseName).Collection(collectionName)
	return collection
}
