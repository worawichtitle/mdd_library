package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func connectMongo() (*mongo.Collection, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://mongodb:27017"))
	if err != nil {
		return nil, err
	}

	db := client.Database("library")
	collection := db.Collection("borrows")

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "borrow_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	_, err = collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		log.Printf("Warning: Failed to create unique index on borrow_id: %v", err)
	} else {
		log.Println("Successfully verified unique index on borrow_id")
	}

	return collection, nil
}
